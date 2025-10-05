package main

const _zshFn = `

redraw-prompt() {
  local precmd
  for precmd in $precmd_functions; do
    $precmd
  done
  zle reset-prompt
}

atuin-fzf-history() {
    local result
    result=$(%v)
    if [[ -z "$result" ]]; then
        return
    fi

    if [[ "$result" == "CHDIR:"* ]]; then
        IFS=$'\t' read result dir cmd  <<< "$result"
        cd "$dir"
        BUFFER="$cmd"
        CURSOR=${#BUFFER}
        redraw-prompt
    else
        # Default action (Enter was pressed): just place the result in the buffer
        BUFFER="$result"
        CURSOR=${#BUFFER}
        zle redisplay
    fi
}

zle -N atuin-fzf-history
bindkey '^r' atuin-fzf-history
`
