/**
 * The app version as the user should read it.
 *
 * Release builds are stamped with the git tag, which already starts with "v"
 * (#21). CI builds carry `git describe` output such as v3.7.0-alpha.2-5-gabc1234,
 * a local build may be stamped "3.6.0", and an unstamped one says "dev". Only a
 * bare version number gets the prefix, so a lone commit hash — what
 * `git describe --always` falls back to without a reachable tag — is shown as
 * it is, even when it happens to start with a digit.
 */
// A dev-channel build (0.0.0-pr.12+abc1234, 0.0.0-main+abc1234) is named by
// what it was built from: "v0.0.0-pr.12+abc1234" says less than "PR #12 · abc1234".
// The backend's parser is the authority (updater.parseBuildIdentity); this
// only words it, so a version it does not recognise falls through unchanged.
const DEV_VERSION = /^v?0\.0\.0-(?:pr\.([1-9]\d{0,8})|main)(?:\+([0-9a-f]{7,40}))?$/;

export function versionLabel(raw: string): string {
  if (!raw) return '…';
  const dev = DEV_VERSION.exec(raw);
  if (dev) {
    const name = dev[1] ? `PR #${dev[1]}` : 'main';
    return dev[2] ? `${name} · ${dev[2].slice(0, 7)}` : name;
  }
  return /^\d+\.\d+/.test(raw) ? `v${raw}` : raw;
}
