
# Action Items

## Bugs

- [ ] When searching for an artist need to able to queue albums in additional to individual songs
- [ ] When picking the Artist button, the albums for an artist should be shown first when drilling down to see what the Artist is associated with, albums should have the plus button tto add a whole album to queue
- [ ] When I add the "Travel" album to a playlist is pickups more than Jake Shimabukuro's "Travel" it also adds ZBS's "Travels with Jack"
- [ ] When I search for Jake or Shimabukuro I am only getting back one of his albums, even when the "Artist" button was previously selected
- [ ] When I type in an artist name under the Albums list, it should return a list of albums by the Artest name, do I need to have query prefix like "artist:Shimabukuro" to do that?
- [ ] The scan action has no visual indicator that is had completed, a progress indicator would be nice to include
- [ ] The sweep action has no visual indicator that it has completed, a progress indicator would be nice to include
- [ ] When I run `audiobox scan` I am seeing errors like ```MACMINI-RD:~ rsdoiel$ audiobox scan
Scanning…
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/01 143 (Kelly's Song) 2011.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/02 Bohemian Rhapsody.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/03 Bring Your Adz.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/04 Boy Meets Girl.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/05 Go For Broke.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/06 Trapped 2010.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/07 Variation On A Dance 2010.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/08 Pianoforte 2010.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/09 Five Dollars Unleaded 2010.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/10 Ukulele Bros..wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/11 Hallelujah.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/12 Bohemian Rhapsody - Live Version.wav: no tags found
```
- [ ] An Album's song list for ```MACMINI-RD:~ rsdoiel$ ls -1 Audio/Music/Albums/Milton-plus-Esperanza/
01 - the music was there.mp3
02 - Cais.mp3
03 - Late September.mp3
04 - Outubro.mp3
05 - A Day in the Life.mp3
06 - Interlude for Saci.mp3
07 - Saci [feat. Guinga].mp3
08 - Wings for the Thought Bird [feat. Elena Pinderhughes & Orquestra Ouro Preto].mp3
09 - The Way You Are.mp3
10 - Earth Song [feat. Dianne Reeves].mp3
11 - Morro Velho [feat. Orquestra Ouro Preto].mp3
12 - Saudade Dos Aviões Da Panair (Conversando No Bar) [feat. Lianne La Havas & Maria Gadú &.mp3
13 - Um Vento Passou (para Paul Simon) [feat. Paul Simon].mp3
14 - Get It By Now.mp3
15 - outro planeta.mp3
16 - When You Dream [feat. Carolina Shorter].mp3
``` doesn't show up when in look for it in the individual web view for Albums, it does show up if I search by artist
- [ ] The Album Travels, ```MACMINI-RD:~ rsdoiel$ ls -1 Audio/Music/Albums/Travels
01 Departure Suite - part I.wav
02 Train Ride.wav
03 Low Rider.wav
04 Travels.wav
05 Interlude 1.wav
06 Passport.wav
07 Hi'ilawe.wav
08 Everything Is Better With You (feat. The Side Order Band).wav
09 Departure Suite - parts II & III.wav
10 'Oama.wav
11 I'll Be There.wav
12 Haven't We Been Here Before.wav
13 Interlude 2.wav
14 Kawika.wav
15 Red-Eye.wav
16 Ichigo Ichie.wav
17 Dinner & A Movie.wav
18 Nada Sousou.wav
19 Early Song.wav
20 Tip Toe.wav
``` is also lists content from Travels with Jack in the Web View (might be an error in the SQL or how the metadata is stored in the SQLite3 database)
- [ ] The folder view in the web UI does not let me drill down into the folders that are inside `Music/Album`
- [ ] The search feature should be clearer about constraints/context of search (searching by album name, searching by artist, etc).

## Next

- [ ] It would be nice to build playlists based on artist name, date of first release (example 1965 to 1975 music)
- [ ] There needs to be a means of importing and exporting a playlist as OPML files
- [ ] An A-Z list jump option or filter needs to be available for Albums, Artists and Titles
- [ ] Sort order for Albums and Titles should use the librarian style ordering where words like "the" and "a" are ignored but still shown in the result, "The Dave Mathews Band" would be sorted as "Dave Mathews Band, The" so it shows up in the "D" list not "T" list
- [ ] Use an OPML file to import/export playlists
- [ ] Revise shuffle constraints, if an audio file is playing it and a user press shuffle for the queue it doesn't change that it is playing or the onces that have played. If no audio is playing and I press shuffle for the queue then the whole queue is shuffled and the selected audio file is set to the new top of list.
- [ ] There needs to a way to explude specific folders from a playlist. 
- [ ] Adding content to the queue is done by pressing the "+" (add to queue) button only
- [ ] There should be a switch or action to allow audiobox to be reachable via HTTP from machines on the same network (share audio button)
- [ ] Add a Folders list (with include/exclude options) in additional to albums, artists and titles
  - This would solve the holiday music problem for when it is not the holiday season
- [ ] Player improvements
  - [ ] The player should always be visible
  - [ ] Remove the "delete" button from the player
  - [ ] The player should show the folder, album (if available ), artitle (if available) and title (if available) of the audio file to be played
- [ ] Search Improvments
  - [ ] searching needs to work for taking you to either the folder list, album list, artist and title depending on matching results.
  - [ ] matching search results to include folders that match, album, artist and title (and indicate what type of match it is)
    - [ ] each matched result should have "+" button for adding that content to the queue and "-" button to remove the matching content from the queue
    - [ ] The individual title or album should have the "+" button to add to the queue
- [ ] Queue improvements
  - [ ] Add a clear button for the queue
  - [ ] For individual items in the queue there can be a "-" (remove from queue) button
  - [ ] As items finish being played from the queue they should be removed from it
  - [ ] The queue doesn't auto-play, first item should be ready in the player but the player doesn't start until the play button is pressed
  - [ ] I should be able to do a shuffle on the queue after getting the content I want to play into it
  - [ ] The queue should indicate an estimate of run time for entries
  - [ ] You should be able to save the queue as a playlist
  - [ ] A playload should be loadable into the queue

