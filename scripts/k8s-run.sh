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

wait_for_job() {
  local deadline=$((SECONDS + 600))
  while ((SECONDS < deadline)); do
    case "$(k get job loadgen -o jsonpath='{.status.succeeded}/{.status.failed}')" in
      1/*) return 0 ;;
      */[1-9]*) return 1 ;;
    esac
    sleep 2
  done
  return 1
}

trap 'k scale deployment/"$lang" --replicas=0 >/dev/null' EXIT
k scale deployment/"$lang" --replicas=1 >/dev/null
k rollout status deployment/"$lang" --timeout=180s >/dev/null
k delete job loadgen --ignore-not-found --wait=true >/dev/null
render "$(dirname "$0")/../k8s/loadgen-job.yaml" | k apply -f - >/dev/null
wait_for_job || { k logs job/loadgen | tail -20; exit 1; }
k logs job/loadgen | sed -n '/^----- BEGIN CSV -----$/,/^----- END CSV -----$/p' | sed '1d;$d' \
  > "results/${RUN_ID}_k8s-${mode}_${lang}.csv"
k logs job/loadgen | grep -v -e '^----- ' -e '^kind,' -e '^interval,' -e '^summary,'
k delete job loadgen --wait=false >/dev/null
