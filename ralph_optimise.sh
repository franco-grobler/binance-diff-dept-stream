#!/bin/bash

# Ralph Wiggum Loop: "I'm helping!"
# Continouosly runs opencode to finish tasks in todo.md

TODO_FILE="todo.md"
MODEL="github-copilot/claude-opus-4.5"
COUNT=0

echo "🚂 Choo-choo-choose to code!"

# Loop as long as there are unchecked boxes "- [ ]" in the todo file
while [ "$COUNT" -le 5 ]; do
  echo "---------------------------------------------------"
  echo "🔍 Found incomplete tasks. I'm helping!"

  # Run opencode
  # - Using quotes for the model as it contains spaces
  # - Passing @todo.md as context (assuming opencode supports @ syntax or we pass it as a file arg)
  opencode run \
    "Read the tasks in @$TODO_FILE. Match everything against @README.md, and find any possible optimisations. For each change, write a benchmark test for before and after results. Write the performance improvements to a optimisations.md file." \
    --model "$MODEL" \
    --file $TODO_FILE \
    --file README.md

  EXIT_CODE=$?

  if [ $EXIT_CODE -ne 0 ]; then
    echo "💥 Uh oh! Opencode tasted like burning (Exit code: $EXIT_CODE)"
    exit $EXIT_CODE
  fi

  COUNT=$((COUNT + 1))
  echo "✅ Task attempt finished. Checking for more..."
  sleep 2
done

echo "🎉 I'm done helping! Go banana!"
