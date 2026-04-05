#!/usr/bin/env bash
# Manual test for statusline.sh — run as: bash .claude/statusline-test.sh
# Outputs raw text with ANSI codes so you can see colors live.

SCRIPT="$(dirname "$0")/statusline.sh"
now=$(date +%s)

echo "=== TEST 1: Full payload (mid-usage, rate limits present) ==="
five_reset=$(( now + 16200 ))   # 4h30m from now
seven_reset=$(( now + 259200 )) # 3d from now
echo '{
  "session_id": "test-123",
  "cwd": "/home/dev/projects/keylint",
  "model": { "id": "claude-sonnet-4-6", "display_name": "Claude Sonnet 4.6" },
  "workspace": {
    "current_dir": "/home/dev/projects/keylint",
    "project_dir": "/home/dev/projects/keylint",
    "added_dirs": []
  },
  "version": "1.0.71",
  "context_window": {
    "total_input_tokens": 50000,
    "total_output_tokens": 5000,
    "context_window_size": 200000,
    "current_usage": { "input_tokens": 45000, "output_tokens": 4000, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0 },
    "used_percentage": 35.5,
    "remaining_percentage": 64.5
  },
  "rate_limits": {
    "five_hour": { "used_percentage": 42.0, "resets_at": '"$five_reset"' },
    "seven_day": { "used_percentage": 18.0, "resets_at": '"$seven_reset"' }
  }
}' | bash "$SCRIPT"

echo ""
echo "=== TEST 2: No messages yet (null percentages, no rate limits) ==="
echo '{
  "session_id": "test-456",
  "cwd": "/home/dev/projects/keylint",
  "model": { "id": "claude-sonnet-4-6", "display_name": "Claude Sonnet 4.6" },
  "workspace": {
    "current_dir": "/home/dev/projects/keylint",
    "project_dir": "/home/dev/projects/keylint",
    "added_dirs": []
  },
  "version": "1.0.71",
  "context_window": {
    "total_input_tokens": 0,
    "total_output_tokens": 0,
    "context_window_size": 200000,
    "current_usage": null,
    "used_percentage": null,
    "remaining_percentage": null
  }
}' | bash "$SCRIPT"

echo ""
echo "=== TEST 3: High usage (red context, red 5h nearly expired) ==="
five_reset_soon=$(( now + 900 )) # 15m from now
seven_reset=$(( now + 518400 ))  # 6d from now
echo '{
  "session_id": "test-789",
  "cwd": "/home/dev/projects/keylint",
  "model": { "id": "claude-opus-4-5", "display_name": "Claude Opus 4.5" },
  "workspace": {
    "current_dir": "/home/dev/projects/keylint",
    "project_dir": "/home/dev/projects/keylint",
    "added_dirs": []
  },
  "version": "1.0.71",
  "context_window": {
    "total_input_tokens": 170000,
    "total_output_tokens": 15000,
    "context_window_size": 200000,
    "current_usage": { "input_tokens": 165000, "output_tokens": 14000, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0 },
    "used_percentage": 87.2,
    "remaining_percentage": 12.8
  },
  "rate_limits": {
    "five_hour": { "used_percentage": 93.0, "resets_at": '"$five_reset_soon"' },
    "seven_day": { "used_percentage": 61.0, "resets_at": '"$seven_reset"' }
  }
}' | bash "$SCRIPT"

echo ""
echo "=== TEST 4: Context gradient showcase ==="
for pct in 10 25 40 50 60 70 80 90 95; do
    echo '{
      "cwd": "/home/dev/projects/keylint",
      "model": { "id": "claude-opus-4-6", "display_name": "Opus 4.6" },
      "workspace": {
        "current_dir": "/home/dev/projects/keylint",
        "project_dir": "/home/dev/projects/keylint"
      },
      "context_window": { "used_percentage": '"$pct"' }
    }' | bash "$SCRIPT"
done

echo ""
echo "=== TEST 5: Countdown timer edge cases ==="
format_countdown() {
    local secs=$1
    if [ "$secs" -le 0 ]; then echo "now"; return; fi
    local days=$(( secs / 86400 ))
    local hrs=$(( (secs % 86400) / 3600 ))
    local mins=$(( (secs % 3600) / 60 ))
    if [ "$days" -gt 0 ]; then echo "${days}d${hrs}h"
    elif [ "$hrs" -gt 0 ]; then echo "${hrs}h${mins}m"
    else echo "${mins}m"; fi
}
echo "16200s  -> $(format_countdown 16200)  (expect: 4h30m)"
echo "2700s   -> $(format_countdown 2700)   (expect: 45m)"
echo "90061s  -> $(format_countdown 90061)  (expect: 1d1h)"
echo "-5s     -> $(format_countdown -5)     (expect: now)"
echo "59s     -> $(format_countdown 59)     (expect: 0m)"
echo "3600s   -> $(format_countdown 3600)   (expect: 1h0m)"
