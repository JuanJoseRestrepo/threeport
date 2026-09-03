## Quickstart

In order to run a local development control plane of threeport, you'll need the
following installed:

* [docker](https://docs.docker.com/get-docker/)
* [kind](https://kind.sigs.k8s.io/)
* [kubectl](https://kubernetes.io/docs/reference/kubectl/)

The following will also need to be installed locally for the relevant development
operations:

* [swag CLI](https://github.com/swaggo/swag) `>=v1.16.2,<v2.0.0` for generating API docs.
* [NATS CLI](https://github.com/nats-io/natscli) for interacting with NATS
  messages used by the control plane.
* [mage](https://magefile.org/) for using mage targets.

First, install the `tptctl` CLI.  There are two ways to do this:

* Build it from this branch's source with mage.  This is the recommended way
  for development, since it guarantees `tptctl` matches the code you're
  working on:

  ```bash
  mage install:tptctl
  ```

  This prints the exact path it installed to, e.g. `tptctl binary installed
  and available at /home/you/go/bin/tptctl`.  If `tptctl version` fails with
  "command not found" afterward, that directory isn't on your `PATH` - add it,
  using the directory from your own command's output (not copied verbatim):

  ```bash
  export PATH="$PATH:/home/you/go/bin"
  ```

  Add that line to your shell profile (e.g. `~/.zshrc` or `~/.bashrc`) to
  persist it across sessions.

* Or install a pre-built release binary by following the [Install tptctl
  guide](../docs/install/install-tptctl.md).  Note this installs the latest
  *released* version, which may not match the in-development code on this
  branch.

Create a local container registry.  This mage target runs a local docker
container to serve as the container registry, so we don't have to wait for
images to be pushed to - and pulled from - a remote registry.

```bash
mage dev:localRegistryUp
```

Build all Threeport control plane images and push them to the local registry:

```bash
mage build:allImagesDev
```

Install Threeport from the newly built images.  `--name` is an arbitrary name
you choose for this control plane instance:

```bash
tptctl up \
  --name dev-0 \
  --provider kind \
  --auth-enabled=false \
  --local-registry \
  --control-plane-image-namespace localhost:5001 \
  --control-plane-image-tag v0.7.0-dev
```

The `--control-plane-image-tag` must match the tag `mage build:allImagesDev`
just pushed to the registry.  That tag comes from `internal/version/version.txt`
(the same file `mage build:allImagesDev` reads via `version.GetVersion()`), so
rather than hardcoding a version that will go stale as this branch is synced
with new releases, read it from that file.  `--control-plane-image-namespace`
is always `localhost:5001` for this local-registry workflow.  Here's the same
command with everything captured as variables, so it stays reusable and never
goes stale:

```bash
CONTROL_PLANE_NAME=<name>
CONTROL_PLANE_IMAGE_NAMESPACE=localhost:5001
CONTROL_PLANE_IMAGE_TAG=$(cat internal/version/version.txt)

tptctl up \
  --name "$CONTROL_PLANE_NAME" \
  --provider kind \
  --auth-enabled=false \
  --local-registry \
  --control-plane-image-namespace "$CONTROL_PLANE_IMAGE_NAMESPACE" \
  --control-plane-image-tag "$CONTROL_PLANE_IMAGE_TAG"
```

This will start a local kind cluster and install the control plane.  You can now
make calls to the API server.

Call the API.  Note this is a different port than the local container registry
(`localhost:5001`) - the API is served on the default http port:

```bash
curl localhost/swagger/index.html
```

Uninstall the local dev control plane:

```bash
tptctl down --name dev-0
```

Or, if you used the variable form above:

```bash
tptctl down --name "$CONTROL_PLANE_NAME"
```

Stop and remove the local container registry:

```bash
mage dev:localRegistryDown
```

