#!/bin/bash

if [ "Xmaster" != "X$TRAVIS_BRANCH" ]; then echo "Current branch is $TRAVIS_BRANCH. We are deploying only from the master branch"; exit 0; fi
if [ "X" != "X$TRAVIS_TAG" ]; then echo "Tags are auto-committed by deploys, so this is already a result of a deploy. Skip deploy this time."; exit 0; fi
if [ "X" == "X$GITHUB_TOKEN" ]; then echo "GITHUB_TOKEN environment variable is not set!"; exit 10; fi
if [ "X" == "X$HOME" ]; then echo "HOME environment variable is not set!"; exit 10; fi

# These package are required for our "github-release-upload.py" script
sudo -H pip install --upgrade pip
sudo -H pip install requests
sudo -H pip install urllib3

echo "Creating Github realase..."
export RELEASE_ID=`python github-release-upload.py`
# Although the Github release ID is expected in this variable, it might contain error codes and messages of the python script above.
# So print it anyway, before we exit the BASH script to see the reason why we exit.
echo $RELEASE_ID
if [ $? -ne 0 ]; then echo "Creating new release failed"; exit 10; fi
if [ "X" == "X$RELEASE_ID" ]; then echo "Release ID was not found in GitHub response"; exit 10; fi
# Check if RELEASE_ID is a valid integer
if [[ $RELEASE_ID = ^-?[0-9]+$ ]]; then echo "Release ID is not a valid integer"; exit 10; fi

echo "Uploading Github realase asset..."
curl --progress-bar \
     -H "Content-Type: application/octet-stream" \
     -H "Authorization: token $GITHUB_TOKEN" \
     --retry 3 \
     --data-binary @$PCKG_DIR/$PCKG_FILE \
     "https://uploads.github.com/repos/$OWNER/$PACKAGE/releases/$RELEASE_ID/assets?name=$PCKG_FILE"
if [ $? -ne 0 ]; then echo "Uploading release asset failed"; exit 10; fi
