Installation for development of **audiobox**
===========================================

**audiobox** audiobox is a go module that for managing audio metadata in an SQLite3 database. It uses "https://github.com/dhowden/tag" module to extract metadata that may be found in the audio file itself.

Quick install with curl or irm
------------------------------

There is an experimental installer.sh script that can be run with the following command to install latest table release. This may work for macOS, Linux and if you’re using Windows with the Unix subsystem. This would be run from your shell (e.g. Terminal on macOS).

~~~shell
curl https://Laboratory.github.io/audiobox/installer.sh | sh
~~~

This will install the programs included in audiobox in your `$HOME/bin` directory.

If you are running Windows 10 or 11 use the Powershell command below.

~~~ps1
irm https://Laboratory.github.io/audiobox/installer.ps1 | iex
~~~

### If your are running macOS or Windows

You may get security warnings if you are using macOS or Windows. See the notes for the specific operating system you're using to fix issues.

- [INSTALL_NOTES_macOS.md](INSTALL_NOTES_macOS.md)
- [INSTALL_NOTES_Windows.md](INSTALL_NOTES_Windows.md)

Installing from source
----------------------

### Required software

- Go &gt;&#x3D; 1.26.2

### Steps

1. git clone https://github.com/Laboratory/audiobox
2. Change directory into the `audiobox` directory
3. Make to build, test and install

~~~shell
git clone https://github.com/Laboratory/audiobox
cd audiobox
make
make test
make install
~~~

