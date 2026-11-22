#!/usr/bin/env bash
# Model-check a task contract's TLA+ model: scripts/check-tla.sh specs/<feature>/tasks/<ID>.tla
# Needs Java and tla2tools.jar (set TLA2TOOLS_JAR). Without them it says so and exits 3, so a contract
# must be marked "not model-checked" instead of claiming the model was verified.
set -u
f="${1:-}"
[ -f "$f" ] || { echo "usage: $0 <model.tla> (with <model>.cfg next to it)"; exit 2; }
cfg="${f%.tla}.cfg"
[ -f "$cfg" ] || { echo "missing $cfg (INIT, NEXT, INVARIANT, CONSTANTS)"; exit 2; }
java -version >/dev/null 2>&1 || { echo "TLC not run: no working java (the macOS /usr/bin/java stub does not count; put a real JDK on PATH). Mark the model 'not model-checked'."; exit 3; }
jar="${TLA2TOOLS_JAR:-}"
[ -f "$jar" ] || { echo "TLC not run: set TLA2TOOLS_JAR to tla2tools.jar. Mark the model 'not model-checked'."; exit 3; }
cd "$(dirname "$f")" && exec java -XX:+UseParallelGC -cp "$jar" tlc2.TLC -config "$(basename "$cfg")" -deadlock "$(basename "$f")"
