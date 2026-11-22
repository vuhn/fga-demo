#!/usr/bin/env bash
# Model-check a task contract's TLA+ model: scripts/check-tla.sh specs/<feature>/tasks/<ID>.tla
# Needs Java and tla2tools.jar. Found automatically: a working java on PATH, else a Homebrew openjdk
# (brew install openjdk); the jar from TLA2TOOLS_JAR, else ~/.local/share/tla/tla2tools.jar. Without them it says so and exits 3, so a contract
# must be marked "not model-checked" instead of claiming the model was verified.
set -u
f="${1:-}"
[ -f "$f" ] || { echo "usage: $0 <model.tla> (with <model>.cfg next to it)"; exit 2; }
cfg="${f%.tla}.cfg"
[ -f "$cfg" ] || { echo "missing $cfg (INIT, NEXT, INVARIANT, CONSTANTS)"; exit 2; }
if ! java -version >/dev/null 2>&1; then
  for d in /opt/homebrew/opt/openjdk/bin /usr/local/opt/openjdk/bin; do
    [ -x "$d/java" ] && PATH="$d:$PATH" && break
  done
fi
java -version >/dev/null 2>&1 || { echo "TLC not run: no working java (the macOS /usr/bin/java stub does not count; put a real JDK on PATH). Mark the model 'not model-checked'."; exit 3; }
jar="${TLA2TOOLS_JAR:-$HOME/.local/share/tla/tla2tools.jar}"
[ -f "$jar" ] || { echo "TLC not run: put tla2tools.jar at ~/.local/share/tla/ or set TLA2TOOLS_JAR. Mark the model 'not model-checked'."; exit 3; }
cd "$(dirname "$f")" && exec java -XX:+UseParallelGC -cp "$jar" tlc2.TLC -config "$(basename "$cfg")" -deadlock "$(basename "$f")"
