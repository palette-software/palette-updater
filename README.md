# Palette Insight Architecture

[![Build Status](https://travis-ci.com/palette-software/palette-updater.svg?token=qWG5FJDvsjLrsJpXgxSJ&branch=master)](https://travis-ci.com/palette-software/palette-updater)
![Palette Insight Architecture](https://github.com/palette-software/palette-insight/blob/master/insight-system-diagram.png?raw=true)

## What is Palette Updater?

Palette Updater is intended to be a service which can remotely and automatically update the [Palette Insight Agent] and Palette Insight Watchdog services, if there is an update available.

Palette Updater runs only on **Windows** at the moment.

Palette Auto Updater consists of 2 components:

* Watchdog
* Manager

### Watchdog

The watchdog component does 4 things:

* makes sure that the [Palette Insight Agent] is running
* performs remote updates
* performs remote start/stop commands
* applies remote configuration on [Palette Insight Agent]

The log file of the Watchdog component is located at `Logs\watchdog.log`.

#### Make sure Palette Insight Agent is running

The Watchdog service *checks regularly* (currently every 5 minutes) whether the [
](https://github.com/palette-software/PaletteInsightAgent) is running or not. If it is not running, Watchdog *restarts the Palette Insight* Agent service, unless it is not commanded to stop by a remote command from the [Insight Server].

#### Check for updates

It is a service which connects to an Insight Server to *check for updates*. If there is an update it performs the update with the help of the Manager component. (We will introduce the Manager component a bit later.) Watchdog is configured by the `Config\Config.yml` file which is relative to the Watchdog's installation folder.

#### Remote start/stop commands

There is another feature of the Watchdog service. It can accept *start/stop commands* from the [Insight Server] and based on those commands it can start/stop the [Palette Insight Agent] service.

#### Apply remote configuration

It is possible to re-configure [Palette Insight Agent] (and as a result the Palette Updater too) remotely via [Insight Server]. Please check the [Insight Server]'s docs how to do that.

### Manager

Manager is a simple application which actually *performs* the update, start or stop operations on the installed [Palette Insight Agent]. Manager is always triggered by the Watchdog service. Actually when the time has come to perform an operation, Watchdog *creates a copy* of the Manager application file, which is called `manager_in_action`, so that even the Manager application can be replaced during an update.

Moreover, when it performs a [Palette Insight Agent] update, it also creates a `Logs\installer.log` file, which contains details about the Agent update installation process. It might come in handy when it comes for debugging the auto-updater feature.

The log file of the Manager component is located at `Logs\manager.log`.

## How do I set up Palette Updater?

At the moment Palette Auto Updater is *bundled with* the [Palette Insight Agent] install package. This means that [Palette Insight Agent] and Watchdog uses the same `Config\Config.yml` file. There is no point in deploying Palette Updater without [Palette Insight Agent].

## Contribution

### Building locally

```bash
go get ./...
go build -v
```

### Testing

```bash
go get -t ./...
go test ./... -v
```

[Insight Server]: https://github.com/palette-software/insight-server
[Palette Insight Agent]: https://github.com/palette-software/PaletteInsightAgent
