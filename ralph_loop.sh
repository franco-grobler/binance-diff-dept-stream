#!/bin/bash

# Ralph Wiggum Loop: "I'm helping!"
# Continouosly runs opencode to finish tasks in todo.md

TODO_FILE="todo.md"
MODEL="github-copilot/claude-opus-4.5"

echo "🚂 Choo-choo-choose to code!"

# Loop as long as there are unchecked boxes "- [ ]" in the todo file
while grep -q "\- \[ \]" "$TODO_FILE"; do
  echo "---------------------------------------------------"
  echo "🔍 Found incomplete tasks. I'm helping!"

  # Run opencode
  # - Using quotes for the model as it contains spaces
  # - Passing @todo.md as context (assuming opencode supports @ syntax or we pass it as a file arg)
  opencode run \
    "Find the first unchecked item in the list. Implement it fully. After implementation, mark the item as checked [x] in @$TODO_FILE. Match everything against @README.md, and add any missing tasks." \
    --model "$MODEL" \
    --file $TODO_FILE \
    --file README.md

  EXIT_CODE=$?

  if [ $EXIT_CODE -ne 0 ]; then
    echo "💥 Uh oh! Opencode tasted like burning (Exit code: $EXIT_CODE)"
    exit $EXIT_CODE
  fi

  echo "✅ Task attempt finished. Checking for more..."
  sleep 2
done

echo "🎉 I'm done helping! Go banana!"
