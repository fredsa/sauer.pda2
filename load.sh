#!/bin/bash

function csv() {
  kind=$1
  lower=$(echo $1 | tr '[A-Z]' '[a-z]')
  upper=$(echo $1 | tr '[a-z]' '[A-Z]')
  rm -f $lower.csv
  cat pda.csv | grep $upper > temp
  row=$(cat temp | head -1)
  (echo "$row"; cat temp | grep -v "$row") > $lower.csv
  rm temp
  url=http://localhost:8080/_ah/remote_api
  url=https://sauer-pda.appspot.com/_ah/remote_api
  set -x
  appcfg.py upload_data \
            --url=$url \
            --kind=$kind \
            --config_file=bulkloader.yaml \
            --filename=$lower.csv \
            --email=archer@allen-sauer.com \
            --batch_size=50 \
            --rps_limit=100 \
            --num_threads=1 \
            --http_limit=20 \
            .
  set +x
  ls -l $lower.csv
}

for kind in Person Contact Address Calendar
do
  csv $kind
done

rm -f bulkloader-log-*
rm -f bulkloader-error-*
rm -f bulkloader-progress-*
