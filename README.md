[![Build Status](https://travis-ci.com/palette-software/palette-updater.svg?token=qWG5FJDvsjLrsJpXgxSJ&branch=master)](https://travis-ci.com/palette-software/palette-updater)

# Palette Auto Updater

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
