#!/bin/bash

#set -x

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

DASHBOARD_UID=eXPEaNnZz
: "${GRAFANA_URL:=http://localhost:3000}"
: "${DASHBOARD_DEFN:=${DIR}/../config/grafana/istio.json}"

echo "      GRAFANA_URL=$GRAFANA_URL"
echo "    DASHBOARD_UID=$DASHBOARD_UID"
echo "   DASHBOARD_DEFN=$DASHBOARD_DEFN"

function get_config {
  if [[ "${DASHBOARD_DEFN}" == https://* ]] || \
     [[ "${DASHBOARD_DEFN}" == http://* ]]; then
    curl -s ${DASHBOARD_DEFN} | cat -
  else
    cat ${DASHBOARD_DEFN}
  fi
}

status=$(curl -Is --header 'Accept: application/json' $GRAFANA_URL/api/dashboards/uid/$DASHBOARD_UID 2>/dev/null | head -n 1 | cut -d$' ' -f2)
echo $status
if [[ "$status" == "200" ]]; then
  echo "Canary Dashboard already defined in $GRAFANA_URL"
  # Could update by copying id, version from current dashboard
fi
else
  echo "Defining canary dashboard on $GRAFANA_URL"
  echo "{ \"dashboard\": $(get_config) }" \
  | jq 'del(.dashboard.id) | del(.dashboard.version)' #\
  | curl --request POST \
    --header 'Accept: application/json' \
    --header 'Content-Type: application/json' \
    $GRAFANA_URL/api/dashboards/db \
    --data @-
  echo ""
fi
