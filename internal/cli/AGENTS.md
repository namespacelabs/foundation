### When adding CLI commands:
- noun should be singular: `nsc instance list`
- flags use underscores: `--max_instance_duration`
- For negation, use `--no_<flag>` (e.g. `--expiration 30d` or to reset `--no_expiration`)
- When adding a new command, add one or more `Examples` to the command definition
