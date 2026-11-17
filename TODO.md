
# Action Items

## Bugs

- [ ] In the list that displays Artists when I click on "T" I get list positioned at entries starting with "A". "U" shows listing starting with "V". The "S" and "R" jump points work fine.

## Next

- [ ] Drop terminal based player, only support playback from web browser
  - [ ] cleanup any code, simplify code base if possible

- [x] Podcast support: listen to MP3s in the Audio/Podcasts directory
  - [x] flag a podcast episode as listened-to
  - [x] automatic sweep that deletes stale listened-to episodes
  - [x] flag an individual episode to be archived, exempting it from the sweep

## Questions

- [ ] Do we still need ffmpeg for if we don't need it for playback in the terminal?


## Modernization (2026-11-16)

To match the workspace standard (knowledge and opml are the models; Go 1.27.3 across repositories). Do one repository at a time; run the tests and `make -n release` after each step.

- [x] Go 1.27.3: change `go 1.26.2` in go.mod, the `Go >=` lines in codemeta.json, README.md and INSTALL.md; run `go mod tidy`, `go vet ./...` and the tests.
- [x] Makefile: add `generate` (cmt for version.go, CITATION.cff, about.md, installers); make `build` stop running cmt. Model: opml/Makefile.
- [x] Makefile: add `check-release` and make `release` run `generate` then `check-release`.
- [x] Makefile: add `check-go-version` (go.mod, codemeta.json and the Go in use agree); `check-release` calls it.
- [x] Drop the armv7l target and 32-bit Pi wording (dropped 2026-10-27).
- [ ] publish.bash/publish.ps1: take knowledge's checked versions (a failed pull no longer publishes nothing silently).
- [ ] release.bash: bring up to fountain's version.
- [ ] release.ps1: take knowledge's (existing-draft check, checksums).
- [ ] website.mak, page.tmpl, add-col-scope.lua, website.ps1: take knowledge's (check `NO_PUBLISH` and nav links).
