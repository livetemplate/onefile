#!/bin/sh
# demo-logs.sh NAME — writes a mix of JSON, logfmt and plain-text log lines,
# some to stderr, until stopped. The default tinkerdown.yaml runs it twice so
# the viewer has something to show before you point it at real logs.
name=${1:-demo}
i=0
while true; do
  i=$((i + 1))
  echo "{\"level\":\"info\",\"msg\":\"$name handled request $i\",\"ms\":$((i % 50))}"
  if [ $((i % 4)) -eq 0 ]; then
    echo "level=warn msg=\"$name slow call\" ms=$((700 + i % 300))" 1>&2
  fi
  if [ $((i % 11)) -eq 0 ]; then
    echo "{\"level\":\"error\",\"msg\":\"$name failed job $i\",\"err\":\"timeout\"}" 1>&2
  fi
  sleep 0.7
done
