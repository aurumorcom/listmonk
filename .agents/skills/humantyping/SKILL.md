---
name: humantyping
description: Provides specialized context, rules, and tools for implementing, configuring, and debugging humantyping. Use this skill whenever modifying humantyping configurations or adding related functionality.
---
# humantyping

## File Tree

```text
humantyping/
├── assets
├── modules
│   └── HumanTyping (https://github.com/Lax3n/HumanTyping)
├── references
├── scripts
└── SKILL.md
```

> **Agent Instructions:** The `modules/` directory contains full source code repositories. Probe is configured for this workspace. Use Probe MCP tools to inspect and search code dynamically across target folder paths instead of raw static AST dumps:
> - `probe search "<query>" [path]` - Search code semantically with Elasticsearch-style syntax.
> - `probe extract <file>:<line>` - Extract complete AST semantic blocks.
> - `probe query "<pattern>"` - Perform AST structural pattern matching.
> - `probe symbols <file>` - List code symbols (functions, classes, constants) in target file.