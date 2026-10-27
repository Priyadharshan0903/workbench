# npm and native releases

Workbench is distributed as `@priyadharshan933/workbench`, licensed under MIT. The npm package
contains a Node launcher and four version-pinned optional runtime packages:

- `@priyadharshan933/workbench-darwin-arm64`
- `@priyadharshan933/workbench-darwin-x64`
- `@priyadharshan933/workbench-linux-arm64`
- `@priyadharshan933/workbench-linux-x64`

Each runtime contains a Go executable with the dashboard embedded, its checksum and dependency
licenses. There are no install hooks, extra runtime downloads or frontend build steps. npm selects
the runtime for the user's OS and CPU; installations must allow optional dependencies.

## Build and check locally

Use Go 1.24+, Node 22.14+ and npm. From the repository:

Release builds use Go 1.26.10, pinned in package.json's `config.goVersion` and both workflows.
The Go tool downloads that compiler when needed. Keep those pins together when updating the
release toolchain; standalone source builds retain the module's Go 1.24 minimum.

```sh
make check
make package-npm
make check-npm
```

`dist/npm/` receives five npm tarballs, four standalone native `.tar.gz` archives,
`release-manifest.json` and `SHA256SUMS`. Only explicit source files and licenses enter these
archives; catalogs, settings, logs, credentials and development tests are excluded from npm.
The source repository's package.json is private to prevent accidental publication from the
checkout. The builder creates the public package manifests inside the tarballs.

`make check-npm` installs the local launcher and matching native tarball offline into a temporary
directory. It checks CLI version/help, real MCP stdio, an explicit catalog directory and use of
the cached binary after deleting the npm installation. It does not start a server or browser.

## First publication

The `.github/workflows/publish.yml` workflow handles packaging and publishing. Other projects'
credentials or trusted-publisher settings do not automatically grant this package access.

1. Push the Workbench changes and run **Package and publish** from GitHub Actions with **publish**
   unchecked. This creates inspectable artifacts without publishing anything.
2. For the initial publication, add a GitHub Actions secret named `NPM_TOKEN` to
   `Priyadharshan0903/workbench`. Use an npm publishing credential for `priyadharshan933` that can
   create the five scoped packages and satisfies the account's publishing/2FA policy. Do not
   commit the credential or put it in a project `.npmrc`.
3. Review the artifacts and native checks. Push a matching version tag, or rerun the workflow
   with **publish** checked, to publish. The initial version is `0.1.0`; its tag is `v0.1.0`.
4. After the packages exist, configure npm trusted publishing for **each of the five packages**:
   GitHub owner `Priyadharshan0903`, repository `workbench`, workflow filename `publish.yml`.
   Allow direct `npm publish` when the npm settings offer an allowed-actions choice. No GitHub
   environment is specified by this workflow.
5. Remove the bootstrap `NPM_TOKEN` secret once trusted publishing is configured and verified.
   The job has `id-token: write` and uses npm 11 for OIDC publishing with provenance.

The npm [trusted-publishing documentation](https://docs.npmjs.com/trusted-publishers) describes
the current account settings and supported runners. Only GitHub-hosted runners are used here.

## Subsequent releases

Update package.json's version and refresh package-lock.json with
`npm install --package-lock-only --ignore-scripts`. Commit the change, run the checks, and push
the matching `vVERSION` tag. The workflow verifies the tag against the manifest before building.

The build runs Go tests, vet, browser syntax and launcher tests. Installed packages are checked
on Linux x64/ARM64 and macOS Intel/Apple Silicon before publishing. Native packages publish first,
then the main launcher. Prerelease versions use npm's `next` tag and GitHub prerelease status;
stable versions use `latest`. GitHub Releases receive both npm and standalone archives/checksums.

Retry a failed publish job against its retained verified artifacts. Already-published versions
are skipped only when npm reports matching tarball integrity. Different contents at an existing
version require a new version. Rebuilding an old version with a different Go toolchain may
produce different artifacts, so preserve the original artifacts for retries.

## Installed data and upgrades

The launcher copies the verified native executable to a stable path in the user data directory:

| Platform | Default directory |
| --- | --- |
| macOS | `~/Library/Application Support/Workbench` |
| Linux | `$XDG_DATA_HOME/workbench`, or `~/.local/share/workbench` |

`WORKBENCH_HOME` overrides this directory. The executable is atomically replaced at
`DIRECTORY/workbench`; project records, `.runtime/` files and active processes are retained.
The stable path supports the existing Go engine reload when a newer npm version is started.
Removing npm's cache or uninstalling the launcher does not delete this directory.

The launcher passes this directory as the default `--root`; an explicit `--root` overrides the
catalog location. Source checkouts and portable archives retain their existing folder-based
defaults. Source catalogs are not imported automatically. Use
`workbench --root /path/to/source list --json` to inspect one, or export/register records
deliberately when moving installations. Keep existing engine installations separate if they
use a different executable path.

`--version` reports the release version independently of the health API's compatibility version.
The dashboard and health response also expose the release version. Native binaries can be used
directly without Node; npm/npx need Node 22.14+. The `package` command exports a source checkout
and its documented files; use GitHub's ready-made native archives for redistribution of an npm
installation.
