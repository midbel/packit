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

#### Comments

Comment are written with a pound character (`#`). Comments can be used at the beginning of the line or after a value.

```
# this is a comment

option value # another comment
```

#### Option

the primary building block of a Packfile is the key/value pair only separated by one or multiple blank characters which together form the option and its value. It is invalid to have an option without at least one value.

```
key value 
``` 

In some cases, option in a Packfile can have multiple values. This can be written as the name of the option and multiple values after delimited  by one or multiple whitespace characters

```
option value1 valueN
```

or by using the same option key multiple times

```
option value1
option valueN
```

An option (the key/value pair) ends with the EOL (`\n` or `\r\n`) character or a comment.

#### Macros

There are two type of macros supported by Packfile. The first type is used for static operations such as defining variables, including external files. The second type enables assignment to value(s) for example by invoking external process or reading from file, etc.

Both macro types follow a syntax similar to option declarations: the macro name is prefixed with a dot (`.`), followed by its argument list.

```
.macro_name <arg1> ... <argN>
```

##### .include

The `.include` macro allows you to insert the contents of an external file directly into the current file at the point where the macro is used. This provides a way to modularize and organize large configuration files by separating stable or reusable definitions from frequently modified content.

Typical use cases include:

* Splitting configuration across multiple files for readability.
* Reusing common or version-controlled configuration blocks.
* Keeping user-specific overrides separate from shared defaults.

Usage:

* The macro must appear at the beginning of a line (preceded only by optional whitespace).
* It accepts exactly one argument: a path to a single file.
* The specified path is interpreted relative to the context directory provided on the command line.

Behavior:

* The included file is parsed and evaluated as if its content appeared inline in place of the `.include`
* Variables declared in the including file (i.e., the file that uses the macro) are accessible from within the included file.

##### .let
##### .env
##### .echo
##### .readfile

The `.readfile` macro read the entire content of the specified file and returns its content as a string.

Usage:

* The macro can only be used where a value is expected. 
* It can not be used as standalone as the `.include macro`
* The macro accepts exaclty one argument: the path to the file to be read
* The specified path is interpreted relative to the context directory provided on the command line.


##### .exec

The .exec macro executes the specified command in a subprocess, passing along all currently defined environment variables. It captures and returns the command’s standard output as a string.

* The macro can only be used where a value is expected. 
* It can not be used as standalone as the `.include macro`
* The macro accepts exaclty one argument: the command to execute

#### Variables

Packfile can use two kind of variables:

1. local variables
2. environment variables

#### Values and their type

In a Packfile, value may be represented in multiple forms determined by its data type. The Packfile supports four primitive value types:

1. Literal: unquoted, raw token interpreted verbatim
2. String: sequence of characters enclosed in single or double quotes
3. Numeric: integer and floating-point representations
4. Boolean: true/false, on/off

The format supports also explicity an object type similar to object in JSON. The supports for an array type is implicit by using an option multiple times and/or using multiple values when defining it.

##### Literal
##### String

Strings must be enclosed in either double quotes (`"`) or single quotes (`'`). Escape sequences are not supported; the content between the delimiters must consist solely of valid UTF-8 characters.

```
doube "this is a string surrounded by double 'quote'! single quote can be used inside"
single 'this is a string surrounded by single "quote"! double quote can be used inside' 
``` 

##### Multiline string

When a string exceeds a single line, Packfile supports multi-line string definitions using a syntax similar to heredoc. This allows strings to span multiple lines without requiring explicit line continuation characters. 

All content between the heredoc opening delimiter and the designated end marker is read and preserved exactly as written. The decoder processes this content literally, without applying any escape mechanisms or transformations.

```
multiline <<EOF
the quick brown fox
jumps over
the lazy dog
EOF
```

##### Template string

Template strings are string literals enclosed in backticks (`` ` ``) that support variable substitution. They enable dynamic content generation by allowing the inclusion of variable references directly within the string.

Within a template string, both local variables and environment variables can be referenced and expanded during evaluation.

```
.let docdir /usr/share/docs/foobar

file {
	source data/Packfile
	target `$docdir/Packfile.sample`
}
```

##### Number

Numeric values in Packfile may be represented as either whole integers or floating-point numbers. Scientific notation (e.g., 1e6) is not supported. For integer values, alternative bases are supported via standard prefixes: binary (`0b`), octal (`0o`), and hexadecimal (`0x`).

```
integer 42
float   0.42
hex     0xdeadcafe
binary  0b010
```

To improve readability, underscores (`_`) may be inserted between digits in both integer and floating-point literals. However, underscores are not permitted at the start or end of the number, nor at the beginning or end of the integral or fractional part in floating-point representations.

```
integer 1_042
float   1_042.1_1
```

##### Boolean

Boolean type is the usual boolean as we all knows. The type has two possible values: `true` or `false`. But the Packfile provides also, two synonyms: `on` and `off`.


##### Object

### Package information

### Files

#### Special case: License

### Change

### Dependency

## Next steps

* automatic dependencies resolution by inspecting ELF binaries
* automatically stripping the binary
* support for `APK` packages
* linting Packfile and/or build packages
* converting existing `.deb`/`.rpm` packages to other packages format
* support for zstd compression