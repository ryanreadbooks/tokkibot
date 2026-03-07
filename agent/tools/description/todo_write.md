Manage a structured task list for complex coding sessions. Helps track progress and keeps the user informed.

## When to Use
- Tasks requiring 3+ steps or careful planning
- User provides multiple tasks or explicitly requests a todo list
- Mark `in_progress` BEFORE starting work (only ONE at a time)
- Mark `completed` IMMEDIATELY after finishing

## When NOT to Use
- Single, trivial tasks (< 3 steps) — just do it directly
- Purely conversational or informational requests

## Task States
| State | Meaning |
|-------|---------|
| `pending` | Not yet started |
| `in_progress` | Currently working (limit: 1) |
| `completed` | Finished successfully |

## Rules
1. **Real-time updates**: Update status as you work, don't batch
2. **Completion integrity**: Only mark completed when FULLY done
   - Keep as `in_progress` if: errors, blockers, partial implementation, failing tests
3. **Actionable items**: Break complex tasks into specific, manageable steps

When in doubt, use this tool — proactive task management ensures thoroughness.