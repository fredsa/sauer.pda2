#!/bin/bash
#
set -ue

VERSION="$( git log -1 --pretty=format:%H )"
if [ -n "$( git status --porcelain )" ]
then
  VERSION="dirty-$VERSION"
fi

git status
echo
echo -e "Hit [ENTER] to continue: \c"
read

SCRIPTS_DIR="$( dirname $0 )"
ROOT_DIR="$( dirname $SCRIPTS_DIR )"

GCLOUD="$(which gcloud)" \
  || (echo "ERROR: gcloud must be in your PATH"; exit 1)
while [ -L "$GCLOUD" ]
do
  GCLOUD="$(readlink $GCLOUD)"
done

BIN_DIR="$(dirname $GCLOUD)"

if [ "$(basename $BIN_DIR)" == "bin" ]
then
  SDK_HOME=$(dirname $BIN_DIR)
  if [ -d $SDK_HOME/platform/google_appengine ]
  then
    SDK_HOME=$SDK_HOME/platform/google_appengine
  fi
else
  SDK_HOME=$BIN_DIR
fi

PROJECT=sauer-pda
echo
echo "Using project: ${PROJECT}"

echo -e "\n*** DEPLOYING ***\n"
$GCLOUD --project "${PROJECT}" app deploy --version "${VERSION}"

echo -e "\n*** SETTING DEFAULT VERSION ***\n"
$GCLOUD --project "${PROJECT}" app versions migrate "${VERSION}"
