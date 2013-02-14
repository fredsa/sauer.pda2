#!/bin/bash
#
set -uex

devappserver2.py \
  --host 0.0.0.0 \
  --skip_sdk_update_check yes \
  . $*
