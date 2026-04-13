#!/usr/bin/env python3
"""Build Python wheels from the Tokkibot Go binary."""

from __future__ import annotations

import argparse
import base64
import csv
import hashlib
import io
import os
import stat
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path
from typing import Dict, List, Optional, Tuple

# Platform mappings: (goos, goarch) -> wheel platform tag
# Current scope: Linux-only wheels.
PLATFORM_MAPPINGS: Dict[str, Tuple[str, str, str]] = {
    "linux-amd64": ("linux", "amd64", "manylinux_2_17_x86_64"),
    "linux-arm64": ("linux", "arm64", "manylinux_2_17_aarch64"),
}

DEFAULT_PLATFORMS = [
    "linux-amd64",
]


def normalize_package_name(name: str) -> str:
    """Normalize package name for wheel filename."""
    return name.replace("-", "_").replace(".", "_").lower()


def normalize_import_name(name: str) -> str:
    """Normalize package name for Python import."""
    return name.replace("-", "_").replace(".", "_").lower()


def compute_file_hash(data: bytes) -> str:
    """Compute SHA256 hash in wheel RECORD format."""
    digest = hashlib.sha256(data).digest()
    encoded = base64.urlsafe_b64encode(digest).rstrip(b"=").decode("ascii")
    return "sha256={0}".format(encoded)


def detect_git_commit(go_dir: Path) -> str:
    """Detect full git commit SHA in go_dir."""
    result = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=str(go_dir),
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        return "unknown"
    commit = result.stdout.strip()
    return commit or "unknown"


def compile_go_binary(
    go_dir: Path,
    go_target: str,
    output_path: str,
    goos: str,
    goarch: str,
    go_binary: str = "go",
    ldflags: Optional[str] = None,
) -> None:
    """Cross-compile Go binary for target platform."""
    env = os.environ.copy()
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    env["CGO_ENABLED"] = "0"

    ldflags_value = "-s -w"
    if ldflags:
        ldflags_value += " " + ldflags

    cmd = [
        go_binary,
        "build",
        "-trimpath",
        "-ldflags={0}".format(ldflags_value),
        "-o",
        output_path,
        go_target,
    ]

    result = subprocess.run(
        cmd,
        cwd=str(go_dir),
        env=env,
        capture_output=True,
        text=True,
    )

    if result.returncode != 0:
        raise RuntimeError(
            "Go compilation failed for {0}/{1}:\n{2}".format(
                goos, goarch, result.stderr.strip()
            )
        )


def generate_init_py(version: str, binary_name: str) -> str:
    """Generate __init__.py content for wheel package."""
    return '''"""Tokkibot Go binary packaged as a Python wheel."""

import os
import stat
import subprocess
import sys

__version__ = "{version}"


def get_binary_path():
    """Return the path to the bundled binary."""
    binary = os.path.join(os.path.dirname(__file__), "bin", "{binary_name}")

    # Ensure binary is executable on Unix-like systems.
    if sys.platform != "win32":
        current_mode = os.stat(binary).st_mode
        if not (current_mode & stat.S_IXUSR):
            os.chmod(binary, current_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

    return binary


def main():
    """Execute the bundled binary."""
    binary = get_binary_path()

    if sys.platform == "win32":
        sys.exit(subprocess.call([binary] + sys.argv[1:]))
    else:
        os.execvp(binary, [binary] + sys.argv[1:])
'''.format(
        version=version, binary_name=binary_name
    )


def generate_main_py() -> str:
    """Generate __main__.py content."""
    return "from . import main\nmain()\n"


def generate_metadata(
    name: str,
    version: str,
    description: str = "Go binary packaged as Python wheel",
    requires_python: str = ">=3.10",
    author: Optional[str] = None,
    author_email: Optional[str] = None,
    license_: Optional[str] = None,
    url: Optional[str] = None,
    readme_content: Optional[str] = None,
) -> str:
    """Generate METADATA file content."""
    lines = [
        "Metadata-Version: 2.1",
        "Name: {0}".format(name),
        "Version: {0}".format(version),
        "Summary: {0}".format(description),
    ]

    if author:
        lines.append("Author: {0}".format(author))
    if author_email:
        lines.append("Author-email: {0}".format(author_email))
    if license_:
        lines.append("License: {0}".format(license_))
    if url:
        lines.append("Home-page: {0}".format(url))

    lines.append("Requires-Python: {0}".format(requires_python))

    if readme_content:
        lines.append("Description-Content-Type: text/markdown")
        lines.append("")
        lines.append(readme_content)

    return "\n".join(lines) + "\n"


def generate_wheel_metadata(platform_tag: str) -> str:
    """Generate WHEEL file content."""
    return """Wheel-Version: 1.0
Generator: tokkibot-build.py
Root-Is-Purelib: false
Tag: py3-none-{0}
""".format(
        platform_tag
    )


def generate_entry_points(entry_point: str, import_name: str) -> str:
    """Generate entry_points.txt content."""
    return """[console_scripts]
{0} = {1}:main
""".format(
        entry_point, import_name
    )


def generate_record(files: Dict[str, bytes]) -> str:
    """Generate RECORD file content."""
    output = io.StringIO()
    writer = csv.writer(output)

    for path, content in files.items():
        if path.endswith("RECORD"):
            writer.writerow([path, "", ""])
        else:
            hash_val = compute_file_hash(content)
            writer.writerow([path, hash_val, len(content)])

    return output.getvalue()


def build_wheel(
    binary_path: str,
    output_dir: str,
    name: str,
    version: str,
    platform_tag: str,
    entry_point: str,
    is_windows: bool = False,
    description: str = "Go binary packaged as Python wheel",
    requires_python: str = ">=3.10",
    author: Optional[str] = None,
    author_email: Optional[str] = None,
    license_: Optional[str] = None,
    url: Optional[str] = None,
    readme_content: Optional[str] = None,
) -> str:
    """Build a wheel file from a compiled binary."""
    normalized_name = normalize_package_name(name)
    import_name = normalize_import_name(name)
    binary_name = entry_point + (".exe" if is_windows else "")

    with open(binary_path, "rb") as f:
        binary_content = f.read()

    files: Dict[str, bytes] = {}
    files["{0}/__init__.py".format(import_name)] = generate_init_py(
        version, binary_name
    ).encode("utf-8")
    files["{0}/__main__.py".format(import_name)] = generate_main_py().encode("utf-8")
    files["{0}/bin/{1}".format(import_name, binary_name)] = binary_content

    dist_info = "{0}-{1}.dist-info".format(normalized_name, version)
    files["{0}/METADATA".format(dist_info)] = generate_metadata(
        name,
        version,
        description=description,
        requires_python=requires_python,
        author=author,
        author_email=author_email,
        license_=license_,
        url=url,
        readme_content=readme_content,
    ).encode("utf-8")
    files["{0}/WHEEL".format(dist_info)] = generate_wheel_metadata(platform_tag).encode(
        "utf-8"
    )
    files["{0}/entry_points.txt".format(dist_info)] = generate_entry_points(
        entry_point, import_name
    ).encode("utf-8")

    record_path = "{0}/RECORD".format(dist_info)
    files[record_path] = b""
    files[record_path] = generate_record(files).encode("utf-8")

    wheel_name = "{0}-{1}-py3-none-{2}.whl".format(
        normalized_name, version, platform_tag
    )
    wheel_path = os.path.join(output_dir, wheel_name)

    with zipfile.ZipFile(wheel_path, "w", zipfile.ZIP_DEFLATED) as whl:
        for file_path, content in files.items():
            if "/bin/" in file_path:
                info = zipfile.ZipInfo(file_path)
                info.external_attr = (
                    stat.S_IRWXU
                    | stat.S_IRGRP
                    | stat.S_IXGRP
                    | stat.S_IROTH
                    | stat.S_IXOTH
                ) << 16
                whl.writestr(info, content)
            else:
                whl.writestr(file_path, content)

    return wheel_path


def resolve_readme_content(readme: Optional[str], go_dir: Path) -> Optional[str]:
    """Read README content if supplied."""
    if not readme:
        return None

    readme_path = Path(readme)
    if not readme_path.is_absolute():
        readme_path = (go_dir / readme_path).resolve()

    if not readme_path.exists():
        raise FileNotFoundError("README file not found: {0}".format(readme_path))

    return readme_path.read_text(encoding="utf-8")


def build_wheels(
    go_dir: str,
    *,
    go_target: str = "./cmd/tokkibot",
    name: Optional[str] = None,
    version: str = "0.1.0",
    output_dir: str = "./dist",
    entry_point: Optional[str] = None,
    platforms: Optional[List[str]] = None,
    go_binary: str = "go",
    description: str = "Tokkibot CLI packaged as Python wheel",
    requires_python: str = ">=3.10",
    author: Optional[str] = None,
    author_email: Optional[str] = None,
    license_: Optional[str] = None,
    url: Optional[str] = None,
    readme: Optional[str] = "README.md",
    ldflags: Optional[str] = None,
    set_version_var: Optional[str] = "main.version",
    set_commit_var: Optional[str] = "main.commit",
) -> List[str]:
    """Build one wheel file for each target platform."""
    go_path = Path(go_dir).resolve()
    if not go_path.exists():
        raise FileNotFoundError("Go directory not found: {0}".format(go_dir))
    if not (go_path / "go.mod").exists():
        raise ValueError("Not a Go module: {0} (no go.mod found)".format(go_path))

    readme_content = resolve_readme_content(readme, go_path)

    if name is None:
        name = go_path.name
    if entry_point is None:
        entry_point = "tokkibot"
    if platforms is None:
        platforms = list(DEFAULT_PLATFORMS)

    combined_ldflags_parts: List[str] = []
    if set_version_var:
        combined_ldflags_parts.append("-X {0}={1}".format(set_version_var, version))
    if set_commit_var:
        commit = detect_git_commit(go_path)
        combined_ldflags_parts.append("-X {0}={1}".format(set_commit_var, commit))
    if ldflags:
        combined_ldflags_parts.append(ldflags)
    combined_ldflags = (
        " ".join(combined_ldflags_parts) if combined_ldflags_parts else None
    )

    out_path = Path(output_dir).resolve()
    out_path.mkdir(parents=True, exist_ok=True)

    built_wheels: List[str] = []
    with tempfile.TemporaryDirectory() as tmp_dir:
        for platform_str in platforms:
            if platform_str not in PLATFORM_MAPPINGS:
                print(
                    "Warning: unknown platform '{0}', skipping".format(platform_str),
                    file=sys.stderr,
                )
                continue

            goos, goarch, platform_tag = PLATFORM_MAPPINGS[platform_str]
            is_windows = goos == "windows"
            binary_ext = ".exe" if is_windows else ""
            binary_path = os.path.join(
                tmp_dir, "{0}_{1}{2}".format(entry_point, platform_str, binary_ext)
            )

            try:
                compile_go_binary(
                    go_path,
                    go_target,
                    binary_path,
                    goos,
                    goarch,
                    go_binary,
                    ldflags=combined_ldflags,
                )
            except RuntimeError as e:
                print("Warning: {0}".format(e), file=sys.stderr)
                continue

            wheel_path = build_wheel(
                binary_path,
                str(out_path),
                name,
                version,
                platform_tag,
                entry_point,
                is_windows=is_windows,
                description=description,
                requires_python=requires_python,
                author=author,
                author_email=author_email,
                license_=license_,
                url=url,
                readme_content=readme_content,
            )
            built_wheels.append(wheel_path)

    return built_wheels


def parse_platforms(platforms_arg: Optional[str]) -> Optional[List[str]]:
    """Parse comma separated platform list."""
    if not platforms_arg:
        return None
    platforms = [item.strip() for item in platforms_arg.split(",")]
    platforms = [item for item in platforms if item]
    return platforms or None


def main() -> int:
    """CLI entry point."""
    parser = argparse.ArgumentParser(
        prog="build.py",
        description="Build Python wheels from Tokkibot Go binaries",
    )
    parser.add_argument(
        "go_dir",
        nargs="?",
        default=".",
        help="Path to Go module directory (default: current directory)",
    )
    parser.add_argument(
        "--go-target",
        default="./cmd/tokkibot",
        help="Go build target package (default: ./cmd/tokkibot)",
    )
    parser.add_argument("--name", default="tokkibot", help="Python package name")
    parser.add_argument(
        "--version", default="0.1.0", help="Package version (default: 0.1.0)"
    )
    parser.add_argument(
        "--output-dir", default="./dist", help="Directory for built wheels"
    )
    parser.add_argument(
        "--entry-point",
        default="tokkibot",
        help="Console script name exposed by the wheel",
    )
    parser.add_argument(
        "--platforms",
        help="Comma-separated platforms, e.g. linux-amd64,linux-arm64",
    )
    parser.add_argument("--go-binary", default="go", help="Path to Go binary")
    parser.add_argument(
        "--description",
        default="Tokkibot CLI packaged as Python wheel",
        help="Package summary",
    )
    parser.add_argument(
        "--requires-python",
        default=">=3.10",
        help="Requires-Python metadata value",
    )
    parser.add_argument("--author", help="Author name")
    parser.add_argument("--author-email", help="Author email")
    parser.add_argument("--license", dest="license_", help="License identifier")
    parser.add_argument("--url", help="Project homepage URL")
    parser.add_argument(
        "--readme",
        default="README.md",
        help="README path for long description (default: README.md)",
    )
    parser.add_argument(
        "--ldflags",
        help="Extra Go linker flags appended after default '-s -w'",
    )
    parser.add_argument(
        "--set-version-var",
        default="main.version",
        help="Go variable injected with --version (default: main.version)",
    )
    parser.add_argument(
        "--set-commit-var",
        default="main.commit",
        help="Go variable injected with current git commit SHA (default: main.commit)",
    )

    args = parser.parse_args()
    platforms = parse_platforms(args.platforms)

    print("Building wheels for Tokkibot...")
    try:
        wheels = build_wheels(
            args.go_dir,
            go_target=args.go_target,
            name=args.name,
            version=args.version,
            output_dir=args.output_dir,
            entry_point=args.entry_point,
            platforms=platforms,
            go_binary=args.go_binary,
            description=args.description,
            requires_python=args.requires_python,
            author=args.author,
            author_email=args.author_email,
            license_=args.license_,
            url=args.url,
            readme=args.readme,
            ldflags=args.ldflags,
            set_version_var=args.set_version_var,
            set_commit_var=args.set_commit_var,
        )
    except (FileNotFoundError, ValueError) as e:
        print("Error: {0}".format(e), file=sys.stderr)
        return 1

    if not wheels:
        print("Error: no wheels were built", file=sys.stderr)
        return 1

    print("Built {0} wheel(s):".format(len(wheels)))
    for wheel in wheels:
        print("  {0}".format(wheel))
    return 0


if __name__ == "__main__":
    sys.exit(main())
