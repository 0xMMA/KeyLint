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
export function versionLabel(raw: string): string {
  if (!raw) return '…';
  return /^\d+\.\d+/.test(raw) ? `v${raw}` : raw;
}
