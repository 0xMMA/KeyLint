#!/usr/bin/env bash
# Claude Code status line for KeyLint
# Saved in .claude/statusline.sh (tracked in repo)
# Also referenced from ~/.claude/settings.json

input=$(cat)

# --- Helpers ---

# Format seconds into compact countdown: 4h30m, 45m, 2d3h, etc.
format_countdown() {
    local secs=$1
    if [ "$secs" -le 0 ]; then
        echo "now"
        return
    fi
    local days=$(( secs / 86400 ))
    local hrs=$(( (secs % 86400) / 3600 ))
    local mins=$(( (secs % 3600) / 60 ))
    if [ "$days" -gt 0 ]; then
        echo "${days}d${hrs}h"
    elif [ "$hrs" -gt 0 ]; then
        echo "${hrs}h${mins}m"
    else
        echo "${mins}m"
    fi
}

now=$(date +%s)

# --- Time ---
time_str=$(date +%H:%M)

# --- Git branch (from cwd in JSON, skip optional locks) ---
cwd=$(echo "$input" | jq -r '.workspace.current_dir // .cwd // empty')
branch=""
if [ -n "$cwd" ] && [ -d "$cwd" ]; then
    branch=$(GIT_OPTIONAL_LOCKS=0 git -C "$cwd" symbolic-ref --short HEAD 2>/dev/null)
    # Compact git status counts via porcelain
    if [ -n "$branch" ]; then
        porcelain=$(GIT_OPTIONAL_LOCKS=0 git -C "$cwd" status --porcelain 2>/dev/null)
        staged=$(echo "$porcelain" | grep -c '^[MADRC]')
        modified=$(echo "$porcelain" | grep -c '^.[MD]')
        untracked=$(echo "$porcelain" | grep -c '^??')
        git_bits=""
        [ "$staged" -gt 0 ] && git_bits="+${staged}"
        [ "$modified" -gt 0 ] && git_bits="${git_bits} ~${modified}"
        [ "$untracked" -gt 0 ] && git_bits="${git_bits} ?${untracked}"
        git_bits=$(echo "$git_bits" | sed 's/^ //')
    fi
fi

# --- Folder (basename of project dir or cwd) ---
proj_dir=$(echo "$input" | jq -r '.workspace.project_dir // .workspace.current_dir // .cwd // empty')
folder=""
if [ -n "$proj_dir" ]; then
    folder=$(basename "$proj_dir")
fi

# --- Model (shortened display name) ---
model=$(echo "$input" | jq -r '.model.display_name // empty')
# Strip common verbose suffixes to keep it compact
model=$(echo "$model" | sed 's/ (.*)//' | sed 's/Claude //' | sed 's/claude //')

# --- Context window ---
used_pct=$(echo "$input" | jq -r '.context_window.used_percentage // empty')

# --- Rate limit countdowns ---
five_h_resets=$(echo "$input" | jq -r '.rate_limits.five_hour.resets_at // empty')
five_h_pct=$(echo "$input" | jq -r '.rate_limits.five_hour.used_percentage // empty')
seven_d_resets=$(echo "$input" | jq -r '.rate_limits.seven_day.resets_at // empty')
seven_d_pct=$(echo "$input" | jq -r '.rate_limits.seven_day.used_percentage // empty')

# --- Build output ---

# ANSI colors
RESET='\033[0m'
DIM='\033[2m'
CYAN='\033[36m'
YELLOW='\033[33m'
GREEN='\033[32m'
MAGENTA='\033[35m'
RED='\033[31m'
DARK_GREY='\033[90m'
ORANGE='\033[38;2;249;115;22m'  # KeyLint accent #f97316

# Context gradient: smooth green → yellow-green → yellow → orange → red
ctx_gradient_color() {
    local pct=$1
    if [ "$pct" -ge 90 ]; then
        echo '\033[38;2;220;38;38m'    # red
    elif [ "$pct" -ge 80 ]; then
        echo '\033[38;2;234;88;12m'    # dark orange-red
    elif [ "$pct" -ge 70 ]; then
        echo '\033[38;2;249;115;22m'   # orange
    elif [ "$pct" -ge 60 ]; then
        echo '\033[38;2;234;179;8m'    # amber
    elif [ "$pct" -ge 50 ]; then
        echo '\033[38;2;202;138;4m'    # dark yellow
    elif [ "$pct" -ge 40 ]; then
        echo '\033[38;2;161;161;20m'   # yellow-green
    elif [ "$pct" -ge 25 ]; then
        echo '\033[38;2;101;163;13m'   # lime
    else
        echo '\033[38;2;34;197;94m'    # green
    fi
}

parts=()

# 1. Branch + git status
if [ -n "$branch" ]; then
    if [ -n "$git_bits" ]; then
        parts+=("$(printf '%b%s %b%s%b' "$GREEN" "$branch" "$YELLOW" "$git_bits" "$RESET")")
    else
        parts+=("$(printf '%b%s%b' "$GREEN" "$branch" "$RESET")")
    fi
fi

# 2. Project name (KeyLint accent orange)
if [ -n "$folder" ]; then
    parts+=("$(printf '%b%s%b' "$ORANGE" "$folder" "$RESET")")
fi

# 3. Model
if [ -n "$model" ]; then
    parts+=("$(printf '%b%s%b' "$MAGENTA" "$model" "$RESET")")
fi

# 4. Context used (smooth gradient)
if [ -n "$used_pct" ]; then
    ctx_int=$(printf '%.0f' "$used_pct")
    ctx_color=$(ctx_gradient_color "$ctx_int")
    parts+=("$(printf '%b%s%b' "$ctx_color" "ctx:${ctx_int}%" "$RESET")")
fi

# 5. 5h countdown timer (dark grey at rest, yellow/red when hot)
if [ -n "$five_h_resets" ] && [ -n "$five_h_pct" ]; then
    remaining_secs=$(( five_h_resets - now ))
    countdown=$(format_countdown "$remaining_secs")
    pct_int=$(printf '%.0f' "$five_h_pct")
    if [ "$pct_int" -ge 80 ]; then
        lim_color="$RED"
    elif [ "$pct_int" -ge 50 ]; then
        lim_color="$YELLOW"
    else
        lim_color="$DARK_GREY"
    fi
    parts+=("$(printf '%b%s%b' "$lim_color" "5h:${countdown}" "$RESET")")
fi

# 6. 7d countdown timer (dark grey at rest, yellow/red when hot)
if [ -n "$seven_d_resets" ] && [ -n "$seven_d_pct" ]; then
    remaining_secs=$(( seven_d_resets - now ))
    countdown=$(format_countdown "$remaining_secs")
    pct_int=$(printf '%.0f' "$seven_d_pct")
    if [ "$pct_int" -ge 80 ]; then
        lim_color="$RED"
    elif [ "$pct_int" -ge 50 ]; then
        lim_color="$YELLOW"
    else
        lim_color="$DARK_GREY"
    fi
    parts+=("$(printf '%b%s%b' "$lim_color" "7d:${countdown}" "$RESET")")
fi

# 7. Clock (last)
parts+=("$(printf '%b%s%b' "$CYAN" "$time_str" "$RESET")")

# Join parts with dim separator
sep="$(printf '%b%s%b' "$DIM" " | " "$RESET")"
result=""
for part in "${parts[@]}"; do
    if [ -z "$result" ]; then
        result="$part"
    else
        result="${result}${sep}${part}"
    fi
done

printf "%b\n" "$result"
