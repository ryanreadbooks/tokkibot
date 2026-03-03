package workspace

import "embed"

//go:embed prompts/*
var PromptsFs embed.FS

//go:embed memory/*
var MemoryFs embed.FS
