
# Action Items

## Bugs

## Next

- [ ] Search results need to show album and artist info, for individual titles it should show the album and artist info too
  - [ ] selecting a an album or artist entry should put the whole list of the album or artist in the pay queue
- [ ] I should be able to do a shuffle on the queue after getting the content I want to play into it
- [ ] The queue to should an estimate of run time for entries
- [X] Add a sweep action that removes entries in the database if they are no longer a corresponding digital object at the path indicated
- [ ] Scan show have a progress indicator and report the number of items added like sweep does.
- [ ] Restructure the directory layout
  - [ ] The configuration file is called audio.yaml always
  - [ ] Use `~/Audio` as the base directory for app, it will hold audio.yaml and audio.db along with directories holding audio files
  - [ ] `audiobox init` will initialize, fix or upgrade an audiobox deployment
    - [ ] create the `~/Audio` directory if it does not exist
    - [ ] create a default `audio.yaml` in `~/Audio` if one doesn't exist
    - [ ] create an audio.db file if one doesn't exist
    - [ ] run scan and sweep to cleanup the audio.db file content
  - [ ] Come up with a sane path structure
    - [ ] Think about something like `<category>/<artist>/<album>/<titles>.<ext>`
      - [ ] Initial categories Music, SpokenWord, Podcast, Books, AudioTheater
      - [ ] artist can be a person or group or group with collaborators (still listed under the group or primary creator)
  - [ ] Directory layout becomes an expression of the metadata and can be one of the navigable items on TUI or Web UI

