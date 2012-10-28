#!/bin/bash
#
set -ue

if [ $( echo "$*" | egrep -- '-V'\|'--version=' >/dev/null; echo $? ) != 0 ]
then
  cat <<EOD

ERROR: Must specify explict '-V <version>' argument, e.g.

   $0 -V <version>

EOD
  exit 1
fi

echo -e "\n*** Rolling back any pending updates (just in case) ***\n"
appcfg.py --oauth2 $* rollback .

echo -e "\n*** DEPLOYING ***\n"
appcfg.py --oauth2 $* update .
