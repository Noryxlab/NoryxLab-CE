# How data leaves a workspace

Every way a file can get out of a workspace, which of them the platform
closes, and which it does not. Written because the question was asked as "can
you disable the download button", and the button turned out to be the least of
it.

Read this before telling anybody, an auditor included, what the platform
prevents.

## The five paths

A workspace is a container running the person's own code, with their data
mounted and a terminal. A file in it can leave by exactly five routes.

| Route | Closed? | By what |
| --- | --- | --- |
| The pod's own network, `curl` and friends | For regulated projects | The egress profile, below |
| The editor's Download entry | Yes | A build-time patch of the image |
| A desktop session's clipboard | Yes, outbound | `-SendCutText 0`, `-noclipboard` |
| The platform's own download and export routes | Optional | `datasets.local_download` |
| The person's browser session | **No** | Nothing. See the last section |

## 1. The pod's network

A workspace, job, scheduled job or application of a project holding regulated
data has no route to the internet. See `WORKLOAD_NETWORK_ISOLATION.md` for the
profiles, the `egress.enforcement` setting, and the two limits that matter: a
NetworkPolicy cannot filter a domain name, and a pod is unpoliced for its
first seconds.

This is the route that does not need a person at the keyboard: a script, a
notebook cell, a scheduled job, a dependency that phones home. It is also the
only one with no volume limit, which is why it was the first to close.

## 2. The editor's Download and Upload entries

**The VS Code image carries a build-time patch.** The explorer's context menu
offered "Download..." and "Upload...", and `openvscode-server` has no option
to turn either off. The request upstream is `microsoft/vscode#296374`, open
since 19 February 2026 with no comment, no milestone and no owner, describing
this exact case.

So the two menu entries are withdrawn when the image is built, by
`environments/noryx-vscode/noryx-withdraw-file-transfer`:

- the editor's web interface is one minified file,
  `out/vs/code/browser/workbench/workbench.js`, served to the browser;
- two calls in it register the entries, both in the menu group
  `5b_importexport`;
- each call is prefixed with `0&&`, which is valid JavaScript inside the comma
  chain they live in, never calls, and moves nothing else. Deleting the calls
  would mean balancing parentheses inside minified code, which is how a patch
  stops being reviewable;
- the anchors are the names that come from the source rather than from the
  minifier: `appendMenuItem`, `ExplorerContext`, the group string;
- **the count is asserted.** Exactly two matches, or the script exits non-zero
  and no image is produced. A patch against code we do not own has to fail
  loudly: the alternative is an image that quietly serves the button again.

Shipped as `noryx-vscode:0.1.5`. The nightly in-cluster rebuild
(`deploy/k8s/base/noryx-environment-rebuild.yaml`) builds from this repository,
so an image rebuilt there carries the patch too; the script is not something a
build host holds locally.

**What it does not do.** It removes the gesture, which is the volume. The
person still has a terminal in that container, so somebody determined keeps
what they can already read. And a drag and drop into the explorer is a
different handler, so an upload by that path still works - the write itself is
still refused wherever the data is mounted read-only.

**Why the editor is not simply replaced.** `code-server` has
`--disable-file-downloads` and `--disable-file-uploads`, and a current base.
It cannot use the Microsoft extension gallery, and Pylance is proprietary and
exists nowhere else; it is a requirement for the people who work here. That
arbitration pins the editor to `openvscode-server`, whose last release is
1.109.5 of 20 February 2026 while VS Code upstream is 1.144. The consequences
are written down in the Enterprise notes rather than rediscovered: no security
updates on that brick since February, and VS Code's "Custom Endpoint" provider
- which would point chat at an internal model gateway - needs 1.122.

## 3. A desktop session's clipboard

Slicer and ParaView are desktops served over VNC. Neither web client ships a
file transfer, so the clipboard was the only channel to the viewer's machine.
It is closed outbound and left open inbound: `-SendCutText 0` on KasmVNC,
`-noclipboard -noprimary` on x11vnc, both verified against the installed
binaries. Pasting a path or an expression into the application still works.

For regulated data this makes the desktop flavours the strongest of the three:
no file channel at all, and no browser file API.

## 4. The platform's own routes

`datasets.local_download` set to `blocked` withdraws the download and
archive-export routes for every dataset and leaves previews in place. HDS
datasets refuse regardless of the setting (ADR-013). It is `allowed` by
default, and it covers the platform's routes only - not the project file
browser's, and not a workspace's own traffic.

## 5. The person's browser session, which is open

A person who may read a file can get it onto their own machine through the
editor, because the bytes travel on the session they already have: the same
connection that paints the editor. No network policy touches it, because the
connection is inbound and its response is part of it.

What bounds this is not a control in the workspace:

- **the right.** Read on regulated data is a decision, and ADR-048, ADR-049
  and ADR-051 are about who holds it. Somebody who must not take data out
  should not hold Read on it.
- **the flavour.** A streamed desktop has no file channel (section 3).
- **the record.** The read is attributed, in a named session, in the audit
  trail - see Enterprise's `docs/AUDIT_TRAIL.md`. What left by `curl` left no
  trace at all; what leaves by a browser leaves a name.

Say this plainly when asked. A platform that claims to prevent a reader from
copying what they read is claiming something no browser-based editor can do.
