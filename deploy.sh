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
echo

os_name="$( uname -s )"
case $os_name in
  Darwin|Linux)
    GCLOUD="gcloud"
    ;;
  MINGW64*)
    GCLOUD="gcloud.cmd"
    ;;
  *)
    echo "ERROR: Unknown OS name for `$os_name`" 1>&2
    exit 1
    ;;
esac

$GCLOUD -v || (echo "ERROR: gcloud must be in your PATH"; exit 1)

echo -e "\n*** DEPLOYING ***\n"
$GCLOUD app deploy --version "${VERSION}" $*
