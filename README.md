# Packit

## Overview

**Packit** allows users to build, read, and verify .deb (Debian packages) and .rpm (Red Hat packages). 

The tool is designed for users working on Linux-based systems that want to build easily their own packages from their sources or binaries. It can also to the inspection of package contents and/or verification of package integrity.

## Features

* **Build Packages**: Generate `.deb` and/or `.rpm` packages from source code, binaries and/or project directories
* **Read Packages**: Inspect  `.deb` and/or `.rpm` packages to view metadata, contents and dependencies
* **Verify Packages**: Ensure the integrity of packages from checksums available in packages
* **Cross-Platform**: Can be used to create `.deb` and `.rpm` packages from Linux and/or Windows from the same command and configuration

## Usage

### Building Packages

To create `.deb` and/or `.rpm`, you can use the following command

```bash
$ packit build -k deb -f Packfile -d dist . 
$ packit build -k rpm -f Packfile -d dist . 
``` 

* **build** is the sub command to create a package from a Packfile - the configuration file used by packit to make the final package
* **-k** specifies the type of package to be build. the supported values at the the time of writing is `.deb` and `.rpm`
* **-f** specifies the location of the Packfile to used. If not provided, the **build** sub command assumes that the file is located in the current working directory and it is called **Packfile**
* **-d** specifies where the final package will be saved once build
* the final argument specifies the context directory. All the paths given in the configuration file are supposed to be relative to this directory

### Reading Packages - show metadata

To show the metadata of an existing package, you can use the command

```bash
$ packit inspect dist/pack-0.1.0.deb
Name        : pack
Version     : 0.1.0
Group       : utils
Priority    : optional
Size        : 11447
Architecture: noarch
Packager    :
Compiler    : go (=1.23.0)
Description : create your package made easy
pack is a command line tool to create your deb and/or rpm packages easily

with pack you can verify the integrity of packages but also check their
content
``` 

### Reading Packages - List files

To show the content of the archive in a package, the `content` command can be used
```bash
$ packit inspect dist/angle-0.1.0.rpm
drwxr-xr-x root     root            0 Mar 29 18:50 /usr
drwxr-xr-x root     root            0 Mar 29 18:50 /usr/bin
-rwxr-xr-x root     root     11462698 Mar 29 18:45 /usr/bin/pack
drw-r--r-- root     root            0 Mar 29 18:50 /usr/share
drw-r--r-- root     root            0 Mar 29 18:50 /usr/share/doc
drw-r--r-- root     root            0 Mar 29 18:50 /usr/share/doc/pack
-rw-r--r-- root     root          776 Mar 18 18:51 /usr/share/doc/pack/Packfile.default
-rw-r--r-- root     root         1049 Mar 29 18:50 /usr/share/doc/pack/copyright
```

### Verifying Packages

To verify the integrity of your package or a third-party package, this can be done with the `verify` command:

```bash
$ packit verify dist/angle-0.1.0.rpm
dist/pack-0.1.0.rpm: package is valid
```

## Next steps

1. automatic dependencies resolution by inspecting ELF binaries
2. automatically stripping the binary
3. support for `APK` packages
4. linting Packfile and/or build packages
5. converting existing `.deb`/`.rpm` packages to other packages format