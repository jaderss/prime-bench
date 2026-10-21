#!/usr/bin/env bash
set -euo pipefail

lang=$1
scenario=$2
case "$scenario" in
  prime) mode=server users=$USERS think=$THINK_TIME ;;
  gateway) mode=gateway users=$GATEWAY_USERS think=$GATEWAY_THINK_TIME ;;
  *) echo "unknown scenario $scenario" >&2; exit 1 ;;
esac

k() { kubectl --context docker-desktop -n prime-bench "$@"; }
render() {
  sed -e "s|\${LANG_NAME}|$lang|g" -e "s|\${SCENARIO}|$scenario|g" -e "s|\${USERS}|$users|g" \
      -e "s|\${THINK_TIME}|$think|g" -e "s|\${DURATION}|$DURATION|g" -e "s|\${RAMP_UP}|$RAMP_UP|g" \
      -e "s|\${SERVER_BITS}|$SERVER_BITS|g" -e "s|\${RUN_ID}|$RUN_ID|g" -e "s|\${LOADGEN_CPUS}|$LOADGEN_CPUS|g" -e "s|\${GO_VERSION}|$GO_VERSION|g" "$1"
}

k scale deployment/"$lang" --replicas=1 >/dev/null
k rollout status deployment/"$lang" --timeout=180s >/dev/null
k delete job loadgen --ignore-not-found --wait=true >/dev/null
render "$(dirname "$0")/../k8s/loadgen-job.yaml" | k apply -f - >/dev/null
k wait --for=condition=complete job/loadgen --timeout=600s >/dev/null || { k logs job/loadgen | tail -20; exit 1; }
k logs job/loadgen | sed -n '/^----- BEGIN CSV -----$/,/^----- END CSV -----$/p' | sed '1d;$d' \
  > "results/${RUN_ID}_k8s-${mode}_${lang}.csv"
k logs job/loadgen | grep -v -e '^----- ' -e '^kind,' -e '^interval,' -e '^summary,'
k delete job loadgen --wait=false >/dev/null
k scale deployment/"$lang" --replicas=0 >/dev/null
