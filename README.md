[![Build Status](https://travis-ci.com/palette-software/palette-updater.svg?token=qWG5FJDvsjLrsJpXgxSJ&branch=master)](https://travis-ci.com/palette-software/palette-updater)

# Palette Auto Updater

Palette Auto Updater is intended to be a service which can remotely and automatically update the Palette Insight Agent and Palette Insight Watchdog services, if there is an update available. Maybe in the future it will be able to update Palette Center and Palette Insight Server components, but it is not in the scope right now.

Palette Auto Updater consists of 2 components:
* Watchdog
* Manager

### Watchdog
It is a service which connects to an Insight Server to *check for updates*. If there is an update it performs the update with the help of the Manager component. (We will introduce the Manager component a bit later.) Watchdog is configured by the `Config\Config.yml` file which is relative to the Watchdog's installation folder.

At the moment Palette Auto Updater is *bundled with* the Palette Insight Agent install package. This means that at the moment Palette Insight Agent and Watchdog uses the same `Config\Config.yml` file.

There is another feature of the Watchdog service. It can accept *start/stop commands* from the Insight Server and based on those commands it can start/stop the Palette Insight Agent service.

Apart from performing updates and commands, the Watchdog service also *checks regularly* (currently every 5 minutes) whether the Palette Insight Agent is running or not. If it is not running, Watchdog *restarts the Palette Insight* Agent service, unless it is not commanded to stop by a remote command from the Insight Server.

The log file of the Watchdog service is `Logs\watchdog.log`.

### Manager
Manager is a simple application which actually *performs* the update, start or stop operations on the installed Palette Insight Agent. Manager is always triggered by the Watchdog service. Actually when the time has come to perform an operation, Watchdog *creates a copy* of the Manager application file, which is called `manager_in_action`, so that even the Manager application can be replaced during an update.

The log file of the Manager application is `Logs\manager.log`. Moreover, when it performs a Palette Insight Agent update, it also creates a `Logs\installer.log` file, which contains details about the Agent update installation process.

## gofmt pre-commit hook:

Go has a formatting tool that formats all code to the official go coding standard, called ```gofmt```. From the [go documentation](https://github.com/golang/go/wiki/CodeReviewComments#gofmt):

> Run gofmt on your code to automatically fix the majority of mechanical style issues. Almost all Go code in the wild uses gofmt. The rest of this document addresses non-mechanical style points.
>
> An alternative is to use goimports, a superset of gofmt which additionally adds (and removes) import lines as necessary.

To use this tool before each commit, create the following ```.git/hooks/pre-commit``` file:

```bash
#!/bin/sh
# Copyright 2012 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# git gofmt pre-commit hook
#
# To use, store as .git/hooks/pre-commit inside your repository and make sure
# it has execute permissions.
#
# This script does not handle file names that contain spaces.

gofiles=$(git diff --cached --name-only --diff-filter=ACM | grep '.go$')
[ -z "$gofiles" ] && exit 0

unformatted=$(gofmt -l $gofiles)
[ -z "$unformatted" ] && exit 0

# Some files are not gofmt'd. Print message and fail.

echo >&2 "Go files must be formatted with gofmt. Please run:"
for fn in $unformatted; do
	echo >&2 "  gofmt -w $PWD/$fn"
done

exit 1
```
