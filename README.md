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

## Packfile

### What is a Packfile

A Packfile is the primary configuration file that contains all the essential information required to build a software package.

The format draws inspiration from both the Nginx configuration syntax and the Universal Configuration Language (UCL), making it both human-readable and flexible.

A typical Packfile is organized into the following sections - some are optional, some have to be present:

1. Basic information about the package, such as its name, version, and maintainer
2. A list of files and directories that should be included in the final package
3. A declaration of other packages or libraries required for this package to run
4. Changelog entries that describe changes across different versions of the package

### Packfile basics

### Package information

### Files

#### Special case: License

### Change

### Dependency

## Next steps

* include files from directory specify in path section
* automatic dependencies resolution by inspecting ELF binaries
* automatically stripping the binary
* support for `APK` packages
* linting Packfile and/or build packages
* converting existing `.deb`/`.rpm` packages to other packages format
* support for zstd compression