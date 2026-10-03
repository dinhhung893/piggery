#!/bin/bash
# run.sh <env dir> <log name> <omp args...>   (env vars PROBE_* pass through)
# Runs omp isolated in <env dir> (own HOME and agent dir), with my pi session's PIGGERY_DISABLED and
# PI_CODING_AGENT removed so omp sees a clean environment. Log: /tmp/pg-omp/<log name>.jsonl
T=$1; L=$2; shift 2
cd $T/proj
rm -f /tmp/pg-omp/$L.jsonl
exec env -u PIGGERY_DISABLED -u PI_CODING_AGENT HOME=$T PATH=$T/bin:$PATH PI_CODING_AGENT_DIR=$T/agent \
  PROBE_LOG=/tmp/pg-omp/$L.jsonl timeout ${RUN_TIMEOUT:-150} omp "$@"
