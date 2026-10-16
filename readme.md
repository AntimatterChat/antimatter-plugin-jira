# Antimatter Jira Plugin

[![Build Status](https://img.shields.io/circleci/project/github/antimatterchat/antimatter-plugin-jira/master)](https://circleci.com/gh/antimatterchat/antimatter-plugin-jira)
[![Code Coverage](https://img.shields.io/codecov/c/github/antimatterchat/antimatter-plugin-jira/master)](https://codecov.io/gh/antimatterchat/antimatter-plugin-jira)
[![Release](https://img.shields.io/github/v/release/antimatterchat/antimatter-plugin-jira)](https://github.com/antimatterchat/antimatter-plugin-jira/releases/latest)
[![HW](https://img.shields.io/github/issues/antimatterchat/antimatter-plugin-jira/Up%20For%20Grabs?color=dark%20green&label=Help%20Wanted)](https://github.com/antimatterchat/antimatter-plugin-jira/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22Up+For+Grabs%22+label%3A%22Help+Wanted%22)

This plugin supports a two-way integration between Antimatter and Jira. Jira Core and Jira Software products, for Server, Data Center, and Cloud platforms are supported. It has been tested with versions 7 and 8.

For versions v3.0 and later of this plugin, multiple Jira instances are supported, configured using [Administrator Slash Commands](https://github.com/antimatterchat/antimatter-plugin-jira#readme).

See the [Antimatter Product Documentation](https://antimatter.chat/docs/admins/apps.html#integrations) for details on installing, configuring, enabling, and using this Antimatter integration.

## Getting started locally

Follow these steps to spin up Antimatter and Jira locally so you can iterate on the plugin with real APIs.

### Prerequisites

- Go **1.23.x**
- Node **18.x** and npm (yarn optional)
- Docker Desktop (for Jira Server + optional Postgres)
- The Antimatter server repo cloned locally alongside this one

### 1. Run Antimatter

Clone the Antimatter server repo, then start the server:

```bash
cd antimatter/server
make run
```

If `./bin/mmctl` is not present yet, build it once so you can tweak config from the command line:

```bash
go build -o bin/mmctl ./cmd/mmctl
```

Configure the server so Jira can reach it and so plugins can be uploaded. Either edit `config/config.json` directly or run the following commands. Note that localhost URLs won't be recognized by the plugin:

```bash
./bin/mmctl --local config set ServiceSettings.SiteURL http://host.docker.internal:8065
./bin/mmctl --local config set PluginSettings.Enable true
./bin/mmctl --local config set PluginSettings.EnableUploads true
./bin/mmctl --local config set ServiceSettings.AllowCorsFrom http://host.docker.internal:8065
./bin/mmctl --local config set ServiceSettings.AllowedUntrustedInternalConnections "localhost,127.0.0.1,::1,host.docker.internal"
```

Restart the server if prompted.

**Reachability tips**

- `host.docker.internal` works on macOS with Docker Desktop. On Linux, specify the gateway explicitly when starting Jira (`docker run --add-host=host.docker.internal:host-gateway ...`) or use your workstation’s LAN IP in both `SiteURL` and the commands above.
- Whichever hostname you choose, open Antimatter in your browser using that same URL as mismatches can trigger WebSocket CORS blocks and slash-command errors.
- If Jira runs on another machine or you need external access, expose Antimatter via a tunnel or a real domain with valid HTTPS, and update the config values accordingly.
- When you create the Application Link in Jira, supply that same externally reachable Antimatter URL. Jira reuses it to call the plugin’s endpoints.
- You can sanity-check connectivity from the Jira container with `docker exec jira curl -I <your-site-url>`.

### 2. Build and install the Jira plugin

```bash
cd ../antimatter-plugin-jira
npm install
make dist

# optionally let the Makefile upload straight to your dev server
export MM_SERVICESETTINGS_SITEURL=http://host.docker.internal:8065
export AM_ADMIN_USERNAME=admin
export AM_ADMIN_PASSWORD=admin
make deploy
```

If you prefer to upload manually, go to **System Console → Plugins → Upload Plugin**, select `dist/jira-*.tar.gz`, then enable the plugin under **Plugin Management**.

For a tighter edit loop, leave `make watch` running – it rebuilds and redeploys whenever you save.

### 3. Launch a local Jira Server in Docker

```bash
docker pull atlassian/jira-software:9.4.30
docker run --name jira \
  -d -p 8080:8080 \
  atlassian/jira-software:9.4.30
```

Open `http://localhost:8080`, request a **Server** evaluation license from [my.atlassian.com](https://my.atlassian.com/), and finish the setup wizard. Wherever the wizard asks for your Antimatter URL, use `http://host.docker.internal:8065` so the container can call back into your dev server.

### 4. Connect the plugin to Jira

Back in Antimatter, confirm the plugin responds to `/jira help`, then install your local instance:

```bash
/jira instance install server http://localhost:8080
```

The command guides you through creating the Application Link inside Jira, exchanging the consumer key/secret, and configuring webhooks. Once that completes, run `/jira connect` to link your Antimatter user, and use `/jira create` or `/jira subscribe` to test the round trip.

At this point you can edit plugin code, let `make watch` redeploy, and immediately exercise the change against a real Jira API.

### Troubleshooting the local setup

- **Plugin immediately disables itself when enabled** – check `logs/mattermost.log`. If you see `please configure the Antimatter server's SiteURL`, set `ServiceSettings.SiteURL` to a reachable address (e.g. `http://host.docker.internal:8065`) and re-enable the plugin.
- **Upload button missing in System Console** – ensure `PluginSettings.EnableUploads` is `true` in `config/config.json`. You may need to restart the server after changing it.
- **`./bin/mmctl` not found** – build it with `go build -o bin/mmctl ./cmd/mmctl` from the `antimatter/server` directory.
- **Jira setup page refuses license** – generate a **Server** evaluation key for the Server ID shown in the wizard via [my.atlassian.com](https://my.atlassian.com/). Make sure you select the Server deployment type, not Cloud.
- **`XSRF Security Token Missing` during Jira setup** – refresh the page or restart the Docker container and re-run the wizard in a fresh browser session; the token expired.
- **Slash commands complain about unreachable Antimatter URL** – Jira is running in a container, so `localhost` refers to the container itself. Use `http://host.docker.internal:8065` (macOS/Linux with Docker Desktop) or your machine’s LAN IP so Jira can reach Antimatter.

## Feature summary

### Jira to Antimatter notifications

#### Channel subscriptions

Notify your team of the latest updates by sending notifications from your Jira projects to Antimatter channels. You can specify which events trigger a notification - and you can filter out certain types of notifications to keep down the noise.

#### Personal notifications: JiraBot

Each user in Antimatter is connected with their own personal Jira account and notifications for issues where someone is mentioned or assigned an issue is mentioned in your own personal Jira notification bot to help everyone stay on top of their assigned issues.

![A personal JiraBot helps keep you on top of your relevant Jira activities](https://github.com/antimatterchat/antimatter-plugin-jira/assets/74422101/e15de4fe-1cb3-47d1-9b0d-538ab82ec91d)

### Manage Jira issues in Antimatter

#### Create Jira issues

- Create Jira issues from scratch or based off of an Antimatter message easily.
- Without leaving Antimatter's UI, quickly select the project, issue type and enter other fields to create the issue.

  ![image](https://user-images.githubusercontent.com/13119842/59113188-985a9280-8912-11e9-9def-9a7382b4137e.png)

#### Attach messages to Jira issues

Keep all information in one place by attaching parts of Antimatter conversations in Jira issues as comments.  Then, on the resulting dialog, select the Jira issue you want to attach it to. You may search for issues containing specific text.

![image](./assets/attach-from-post.png)

![image](https://user-images.githubusercontent.com/13119842/59113267-b627f780-8912-11e9-90ec-417d430de7e6.png)

#### Transition Jira issues

Transition issues without the need to switch to your Jira project. To transition an issue, use the `/jira transition <issue-key> <state>` command.

For instance, `/jira transition EXT-20 done` transitions the issue key **EXT-20** to **Done**.

![image](https://user-images.githubusercontent.com/13119842/59113377-dfe11e80-8912-11e9-8971-f869fa123366.png)

#### Assign Jira issues

Assign issues to other Jira users without the need to switch to your Jira project. To assign an issue, use the `/jira assign` command.

For instance, `/jira assign EXT-20 john` transitions the issue key **EXT-20** to **John**.

## Releases

Pushing a `v*` tag builds the app and publishes it as a
[GitHub release](https://github.com/Antimatterchat-Community/antimatter-plugin-jira/releases) ([release workflow](.github/workflows/release.yml),
using the organisation's shared
[app release workflow](https://github.com/AntimatterChat/.github/blob/main/.github/workflows/app-release.yml)).
The version is the tag without its `v`. Tags with a `-`, such as `v1.2.0-rc.1`, are published as prereleases. Each release has:

- `jira-<version>.tar.gz`: the app bundle, with the server for Linux (amd64, arm64), macOS (amd64, arm64) and Windows (amd64)
- `jira-<version>.tar.gz.sig`: its detached signature with the Antimatter plugin signing key
- `SHA256SUMS`: the checksums of the release files

Install the bundle with **Upload Plugin** in **System Console > Plugins > Plugin Management** or
`amctl plugin install-url <bundle URL>` (both need `AM_PLUGINSETTINGS_ENABLEUPLOADS=true`), see
[Installing other apps](https://antimatter.chat/docs/admins/apps.html). A release without `.sig`
files is unsigned (its notes say so): a server with Require Plugin Signature
(`PluginSettings.RequirePluginSignature`) on refuses it. Running the workflow by hand (*Actions* → *Release* → *Run workflow*)
builds the bundle as a workflow artifact, and as a draft prerelease if you tick the box; builds
of untagged commits are versioned after the latest tag and the commit.

## Feedback and support

Found a bug in the Jira plugin, or missing a feature? Please [open an
issue](https://github.com/AntimatterChat/community/issues/new/choose) in the [Antimatter community
repository](https://github.com/AntimatterChat/community), where the feedback for every part of
Antimatter is gathered. Questions and ideas are welcome in its
[Discussions](https://github.com/AntimatterChat/community/discussions). Please report security
problems privately, as explained in the [security
policy](https://github.com/AntimatterChat/.github/blob/main/SECURITY.md).

## License

This repository is licensed under the [Apache 2.0 License](LICENSE.txt).

## Development

This plugin contains both a server and web app portion. Read our documentation about the [Developer Workflow](https://antimatter.chat/docs/developers/apps.html) and [Developer Setup](https://antimatter.chat/docs/developers/building.html) for more information about developing and extending plugins.

### Releasing new versions

The version of a plugin is determined at compile time, automatically populating a `version` field in the [plugin manifest](plugin.json):
* If the current commit matches a tag, the version will match after stripping any leading `v`, e.g. `1.3.1`.
* Otherwise, the version will combine the nearest tag with `git rev-parse --short HEAD`, e.g. `1.3.1+d06e53e1`.
* If there is no version tag, an empty version will be combined with the short hash, e.g. `0.0.0+76081421`.

To disable this behaviour, manually populate and maintain the `version` field.

## How to Release

To trigger a release, follow these steps:

1. **For Patch Release:** Run the following command:
    ```
    make patch
    ```
   This will release a patch change.

2. **For Minor Release:** Run the following command:
    ```
    make minor
    ```
   This will release a minor change.

3. **For Major Release:** Run the following command:
    ```
    make major
    ```
   This will release a major change.

4. **For Patch Release Candidate (RC):** Run the following command:
    ```
    make patch-rc
    ```
   This will release a patch release candidate.

5. **For Minor Release Candidate (RC):** Run the following command:
    ```
    make minor-rc
    ```
   This will release a minor release candidate.

6. **For Major Release Candidate (RC):** Run the following command:
    ```
    make major-rc
    ```
   This will release a major release candidate.

### Environment

Join the [AntimatterChat/community Discussions](https://github.com/AntimatterChat/community/discussions) to discuss any questions.

Read our documentation about the [Developer Workflow](https://antimatter.chat/docs/developers/apps.html) and [Developer Setup](https://antimatter.chat/docs/developers/building.html) for more information about developing and extending plugins.

This plugin supports both Jira Server (self-hosted) and Jira Cloud instances. There can be slight differences in behavior between the two systems, so it's best to test with both systems individually when introducing new webhook logic, or adding a new Jira API call.

To test your changes against a local instance of Jira Server, you need [Docker](https://docs.docker.com/install) installed, then you can use the `docker-compose.yml` file in this repository to create a Jira instance. Simply run `docker-compose up` in the directory of the repository, and a new Jira server should start up and be available at http://localhost:8080. It can take a few minutes to start up due to Jira Server's startup processes. If the container fails to start with `exit code 137`, you may need to increase the amount of RAM you are allowing docker to use.

To test your changes against a Jira Cloud instance, we recommend starting a 14-day trial, if you don't have a Jira project to test against. More information can be found here: https://www.atlassian.com/software/jira/try.
