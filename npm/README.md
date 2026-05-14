# VibeCoding

A terminal-based AI coding assistant.

## Installation

```bash
npm install -g vibecoding-installer
```

## Usage

```bash
# Start interactive mode
vibecoding

# With initial prompt
vibecoding "Explain this codebase"

# Non-interactive (print mode)
vibecoding -p "Write a hello world in Go"

# Specify provider and model
vibecoding --provider openai --model gpt-4o

# Change mode
vibecoding --mode plan    # Read-only planning
vibecoding --mode agent   # Standard (default)
vibecoding --mode yolo    # Full access
```

## Configuration

Set your API key:

```bash
# Anthropic
export ANTHROPIC_API_KEY=sk-ant-...

# OpenAI
export OPENAI_API_KEY=sk-...
```

## More Information

- [GitHub Repository](https://github.com/startvibecoding/vibecoding)
- [Documentation](https://github.com/startvibecoding/vibecoding/tree/main/docs)

## License

MIT
