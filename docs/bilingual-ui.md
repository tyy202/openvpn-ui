# Bilingual UI

## Scope and design

Add English and Simplified Chinese to the existing management UI. Both the login
page and navigation expose a language selector. A saved browser preference wins;
otherwise Chinese browser locales use Chinese, and other locales use English.
Storage failures must not prevent switching languages in the current page.

Use explicit `data-i18n` markers and a local dictionary, without scanning or
translating arbitrary user content. English source text is the fallback. Preserve
form values, certificate identities, logs, configuration directives, and editor
contents. Translate known application messages, preserving their parameters.
Switch in place without reloading or losing unsaved form edits.

## Implementation plan

- Add browser tests for locale selection, persistence, switching, fallback,
  dynamic messages, and preserving form values and user content.
- Add a dependency-free language runtime and Chinese dictionary under static/js.
- Mark literal template text and display attributes; wire the selector into both
  page entry points, localize confirmation dialogs and theme labels.
- Verify dictionary coverage and browser behavior; document deployment and limits.

## Maintenance

`static/js/i18n-zh.js` maps original English strings to Simplified Chinese.
Use `data-i18n="English text"` on text-only elements, and
`data-i18n-title`, `data-i18n-placeholder`, or `data-i18n-aria-label` for attributes.
Use `data-i18n-message` only on application messages, never arbitrary user data.
Do not mark inputs' values, configuration textareas, code blocks, or log content.
For dynamic labels call `OpenVPNUILanguage.setText(element, englishText)`.

The assets are packaged with the existing static/ and views/ directories.
Rebuild the UI image to deploy source changes: a Compose service still pointing
at the upstream `d3vilh/openvpn-ui:latest` image will not include local edits.

Browser tests: install Playwright in your development environment (not required
by the application), then run `node --test tests/i18n.test.cjs`. The tests use
headless Chromium (or Microsoft Edge when available), a temporary loopback HTTP
server, and synthetic UI data; no VPN or database is required.
