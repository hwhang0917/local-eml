# Contributing

Thanks for taking an interest in Local Eml. Issues and pull requests are both welcome; this page tells you what makes them easy to act on.

## Reporting a bug

Open a [GitHub issue](https://github.com/hwhang0917/local-eml/issues/new) with:

- What you did, what you expected, and what happened instead.
- Your OS and the output of `local-eml version`.
- How you run it: the installer (background service) or double-click app mode.
- The relevant lines from the log at `~/.local-eml/logs/local-eml.log` (`%USERPROFILE%\.local-eml\logs\` on Windows). **The log contains subjects and addresses from your mail — redact anything private before pasting.**
- For parsing or rendering bugs, a sample `.eml` that reproduces it, with personal content replaced. A message you wrote to yourself is ideal.

For a security issue, do not open a public issue. Use the repository's **Security → Report a vulnerability** form instead.

## Suggesting a feature

Open an issue and describe the problem you are trying to solve, not only the solution. A few things are deliberate and unlikely to change, so a proposal that fits them has a much better chance:

- It is an **archive**, not a mail client: no composing, sending, or replying, and IMAP stays read-only.
- It only ever listens on `127.0.0.1`.
- It ships as **one pure-Go binary**. Anything that needs CGO breaks the cross-compile matrix in `release.yml`.
- Nothing about your mail leaves your machine.

## Pull requests

1. Open an issue first for anything beyond a small fix, so the approach is agreed before you spend time on it.
2. Fork, branch from `main`, and follow [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) to build and run.
3. Before pushing:

   ```bash
   make check                      # gofmt + go vet + go test ./... -race
   cd web && npx vue-tsc --noEmit  # type-check the SPA
   ```

4. Add or update a test for behaviour you change. Backend tests live next to the code they cover.
5. Keep `web/src/locales/en.json` and `ko.json` in lockstep: every new key goes into both. If you cannot write the Korean, add the English text under the Korean key and say so in the PR; it gets translated before merge.
6. Use [Conventional Commits](https://www.conventionalcommits.org/): `type(scope): message`, for example `feat(import): outlook .pst archives` or `fix(viewer): keep scroll position on thread switch`.
7. Keep PRs focused. A refactor and a feature are two PRs.

Do not commit `web/dist/` output or `VERSION` bumps; releases are cut by the maintainer.

## Adding a language

Translations are very welcome. To add a locale:

1. Copy `web/src/locales/en.json` to `<code>.json` and translate the values, keeping every key.
2. Register the code in `web/src/i18n.ts`: the `Locale` type, the `messages` map, and `detectInitial` (stored value and browser-language detection).
3. Add the option to the language picker in `web/src/pages/settings/LocalePage.vue`.
4. Add the "remote image blocked" labels for the language to `blockedLabels` in `internal/sanitize/html.go`, with a test case.
5. If you can, translate `README.md` to `README_<code>.md` and link it from the language line at the top of each README.

## License

By contributing you agree that your work is released under the [GNU GPL v3.0](LICENSE), the same license as the project.
