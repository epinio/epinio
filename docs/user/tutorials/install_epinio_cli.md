#  Installation of the Epinio CLI

If not done already, refer to [System Requirements](https://github.com/epinio/epinio#system-requirements).

## Install Dependencies

Follow these [steps](./install_dependencies.md) to install dependencies.

## Install Epinio CLI

### Download the Binary

Find the latest version at [Releases](https://github.com/epinio/epinio/releases).
Run one of the commands below, and replace \<epinio-version\> with the version of your choice, e.g. "v0.0.21".

##### Linux

```bash
curl -o epinio -L https://github.com/epinio/epinio/releases/download/<epinio-version>/epinio-linux-amd64
```

##### MacOS

```bash
curl -o epinio -L https://github.com/epinio/epinio/releases/download/<epinio-version>/epinio-darwin-amd64
```

##### Windows

```bash
 curl -LO https://github.com/epinio/epinio/releases/download/<epinio-version>/epinio-windows-amd64.exe
```

### Make the Binary Executable

For example on Linux and Mac:

```bash
chmod +x epinio
```

Move the binary to your PATH

```bash
sudo mv ./epinio /usr/local/bin/epinio
```

### Verify the Installation

Run e.g. `epinio version` to test the successful installation.

```bash
> epinio version
Epinio Version: v0.0.21
Go Version: go1.16.7
```
