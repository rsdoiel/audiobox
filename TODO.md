
# Action Items

## Bugs

## Next

- [ ] Add a sweep action that removes entries in the database if they are no longer a corresponding digital object at the path indicated
- [ ] Restructure the directory layout
  - [ ] The configuration file is called audio.yaml always
  - [ ] Use `~/Audio` as the base directory for app, it will hold audio.yaml and audio.db along with directories holding audio files
  - [ ] `audiobox init` will initialize, fix or upgrade an audiobox deployment
    - [ ] create the `~/Audio` directory if it does not exist
    - [ ] create a default `audio.yaml` in `~/Audio` if one doesn't exist
    - [ ] create an audio.db file if one doesn't exist
    - [ ] run scan and sweep to cleanup the audio.db file content
  - [ ] Come up with a sane path structure
    - [ ] `<category>/<artist>/<album>/<titles>`
      - [ ] artist can be a person or group or group with collaborators (still listed under the group or primary creator)
    - [ ] Initial categories Music, SpokenWord, Podcast, Books, AudioTheater
  - [ ] Directory layout becomes an expression of the metadata and can be one of the navigable items on TUI or Web UI

