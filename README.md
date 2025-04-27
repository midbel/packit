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

There are two additional options to control how the package is built. You can choose between:

1. Building the full package (binary and documentation) together.
2. Building only the documentation package via the **--only-docs** option.
3. Building the binary and documentation packages separately thanks to the **--split-docs** option.

The default behaviour is to build the full package binary and documentation included.

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

##### .macro

The .macro directive allows you to define custom macros, serving as reusable shortcuts that can substitute calls to the .exec macro along with their arguments.

Usage:

* the macro must only appear at the beginning of a line (preceded) only by optional whitespace
* it takes exactly two arguments:
  1. the identifier of the customed macro that can be used later in the Packfile 
  2. the command string that will be executed when calling the customed macro

##### .let

The `.let` macro defines a new variable within the current Packfile's scope. Once defined, value of variable can be retrieved from its identifier with the syntax `$ident`

Usage:

* the macro must only appear at the beginning of a line (preceded) only by optional whitespace
* it takes exactly two arguments:
  1. the identifier of the variable and the second 
  2. its value which may be of any supported "primitive" type

Limitations:

* variables defines with the `.let` macro are immutable. Once defined, their values can not be reassigned or modified

##### .env

The `.env` macro similarly to the `.let` macro, but instead of defining a local variable scoped to the current Packfile, it declares an environment variable. Environment variables defined using .env are propagated to any subprocesses or commands executed during evaluation.

Usage:

* the macro must only appear at the beginning of a line (preceded) only by optional whitespace
* it takes exactly two arguments:
  1. the identifier of the variable and the second 
  2. its value which may be of any supported "primitive" type

##### .echo

The .echo macro is a utility directive primarily intended for debugging purposes (eg: logging the content of variables) during packfile parsing by the decoder.

Usage:

* the macro must only appear at the beginning of a line (preceded) only by optional whitespace
* it can take multiple arguments all are printed on stdout 

##### .readfile

The `.readfile` macro read the entire content of the specified file and returns its content as a string.

Usage:

* The macro can only be used where a value is expected. 
* It can not be used as standalone as the `.include macro`
* The macro accepts exaclty one argument: the path to the file to be read
* The specified path is interpreted relative to the context directory provided on the command line.


##### .exec/.shell

The `.exec` macro executes the specified command in a subprocess, passing along all currently defined environment variables. It captures and returns the command’s standard output as a string. Similar to the `.exec` macro, the `.shell` macro does the same thing except that it run the new process in a sub shell instead of directly as a subprocess.

Usage:

* The macro can only be used where a value is expected. 
* It can not be used as standalone as the `.include macro`
* The macro accepts exaclty one argument: the command to execute

##### .git

The .git macro lets you access basic information from the Git repository where the Packit command is run, including:

* user
* email
* latest tag
* current branch
* `origin` remote URL

#### Variables

Two kind of variables can be used inside a Packfile. 

Local variables are denoted by a dollar sign (`$`) followed by an identifier. They are defined using the .let macro within a Packfile. Local variables are scoped to the Packfile in which they are declared, as well as any Packfiles included by it.

```
.let foo foobar
.let answer 42

package `$foo-$answer`
```

Environment variables are identified by an at sign (`@`) followed by an identifier. Environment variables are the one accessible to the `packit` command at runtime or defined within the Packfile using the `.env` macro. Like local variables, environment variables are visible within the defining Packfile and its included Packfiles, but they are also accessible to any subprocesses invoked through the `.exec` macro.

```
.env USER foobar
.env PASS foobar
version .exec `curl -u @USER:@PASS https://@HOSTNAME:8080/version`
```

##### Predefined Variables

The Packfile defines a set of built-in variables that are always available. These variables include:

* arch64: amd64
* arch32: i386
* noarch: noarch
* archall: all
* etcdir: etc
* vardir: var
* logdir: var/log
* optdir: opt
* bindir: bin
* usrbindir: usr/bin
* docdir: usr/share/doc

#### Values and their type

In a Packfile, value may be represented in multiple forms determined by its data type. The Packfile supports four primitive value types:

1. Literal: unquoted, raw token interpreted verbatim
2. String: sequence of characters enclosed in single or double quotes
3. Numeric: integer and floating-point representations
4. Boolean: true/false, on/off

The format supports also explicity an object type similar to object in JSON. The supports for an array type is implicit by using an option multiple times and/or using multiple values when defining it.

##### Literal

A Literal value is primarily a raw token composed of any combination of characters—letters, digits, punctuation—delimited by whitespace. The most common use case for literals is as option keys.

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

The object type ressembles a JSON object, enclosed in `{` and `}` sign. Within the object, the Packfile specific key/value pair is used as described above. For certains cases of object - see below - keys may appear multiple times.

### Package information

At the top level of the file — outside of any object — options define general metadata and configuration for the package to be built by `packit`. These options influence how the package is described and how it behaves during installation, removal, or other package lifecycle stages.

Below is a list of supported top-level options:

* **package/name**: the name of the package.
* **version**: the version string of the package.
* **release** (rpm only): the release number of the package build.
* **summary**: a short one-line summary describing the package.
* **description/desc**: A longer, more detailed description of the package.
* **distrib**: The target distribution for the package.
* **vendor**: The name of the package vendor or author.
* **section/group**: The software section or category under which the package should be listed.
* **priority**: Indicates the priority or importance of the package (e.g., optional, required).
* **home/url**: The homepage or project URL associated with the package.
* **type** (deb only): The DEB package type (e.g., deb, udeb).
* **os**: Target operating system.
* **arch/architecture**: Target architecture(s) for the package (e.g., x86_64, arm64).
* **compiler**: name and/or version of the compiler/tool used to build the binary included in the package
* **maintainer**: Maintainer's name and/or email.
* **pre-install**: Script or command to run before installation.
* **post-install**: Script or command to run after installation.
* **pre-remove**: Script or command to run before removal.
* **post-remove**: Script or command to run after removal.
* **check-package** (rpm only): A command or flag used to validate the package after it has been built.
* **setup**: Custom setup script to be executed prior to build the package itself
* **teardown**: Custom teardown script to be executed during cleanup or after package have been build.

Depending on the type of package being built (RPM or DEB), certain options may be required, optional, or ignored. packit does not enforce the use of all available options, allowing flexibility based on the packaging format and specific needs.

#### Note on compiler option

The `compiler` option can be specified in two different forms as illustred below.

The object syntax can be used:

```
compiler {
  name    go
  version 1.24.1
}
```

But the key/value pair can also be used.

```
compiler go 1.24.1
```

#### Note on maintainer option

The `maintainer` option can be specified in two different forms as illustrated.

The object syntax can be used:

```
maintainer {
  name    foobar
  email   noreply@foobar.org
}
```

But the key/value pair can also be used.

```
maintainer foobar noreply@foobar.org
```

### Files

File objects are defined using the top-level file option and specify which files should be included in the final package. These entries determine how files from the context directory are mapped, handled, and installed within the package.

Each file object supports the following options:

* **source**: Specifies the path or glob pattern (relative to the context directory) that matches the source file(s) to include.
* **target**: Defines the destination path within the package where the file(s) should be installed.
* **ghost** (rpm only): Marks the file as a "ghost" file. Ghost files are not physically included in the package payload but are expected to be created or managed by scripts at runtime.
* **doc** (rpm only): Identifies the file as documentation. These files are installed into the appropriate documentation directory (e.g., /usr/share/doc).
* **license** (rpm only): Flags the file as a license file, which may be used by RPM tools to extract license metadata.
* **readme** (rpm only): Tags the file as a README. Like doc, it may be placed in a standard documentation path and used for informational purposes.
* **conf/config**: Marks the file as a configuration file. During package upgrades, configuration files are preserved if modified.
* **perm**: Sets the file permissions for the installed file (e.g., 0644, 0755).

Multiple file objects can be defined within a single Packfile.

### License

License can be specified in two different forms. First the object syntax can be used with the following options:

* **text**: the literal text of the license
* **file**: the file containing the license text relative to the context directory
* **type**: the type of license (e.g.: mit, gpl)

Both text and file can be used within the same object but only the last value will be used.

Using the simple key/value syntax, the name of a pre-defined template can be used:

```
license mit
```

Typically, the license option is used only once in a Packfile. While specifying it multiple times does not produce an error, only the last occurrence will be used when embedding license information into the package.

### Change

The Change options are used to define changelog entries that will be included in the final built package. Each entry documents updates made in a specific version of the package.

Multiple Change entries can be defined to reflect the package's history across versions.

* **summary**: A concise, one-line summary describing the key updates or purpose of the release.
* **change**: A list of detailed changes, fixes, or improvements introduced in the version. Multiple values can be provided.
* **version**: The package version associated with the listed changes.
* **date**: The release date for the given version.
* **maintainer**: The name (and optionally email) of the maintainer who built the package with these changes.

### Dependency

The Depends options are used to declare package dependencies—other packages that must be installed for the current package to function correctly. These dependencies ensure that required libraries, tools, or components are available at install time or runtime.

Multiple Depends entries can be defined to specify a list of required packages.

* **package**: The name of the required package.
* **type**: The type of dependency (e.g., require, suggests, recommands, conflicts). This helps distinguish between dependencies needed for building the package versus running it.
* **arch**: Architecture-specific constraint for the dependency (e.g., x86_64, arm64). Useful when a dependency is only needed on certain platforms.
* **version**: A version requirement or constraint for the dependency. This defines the acceptable version range for the dependency to be considered valid. Contraints are given via `eq`, `lt`, `le`, `gt`, `ge`, `ne`

## Next steps/TODOS

* build hooks (before/after archive, before/after metadata, ...)
* automatic dependencies resolution by inspecting ELF binaries
* automatically stripping the binary
* support for `APK` packages
* linting Packfile and/or build packages
* converting existing `.deb`/`.rpm` packages to other packages format
* support for zstd compression