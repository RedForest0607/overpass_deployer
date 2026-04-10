# overpass-deployer release install

This archive contains the `deploy` CLI binary.

Quick install:

```bash
tar -xzf deploy_<version>_<os>_<arch>.tar.gz
chmod +x deploy
./deploy version
```

Recommended install path:

```bash
mkdir -p ~/bin
mv ./deploy ~/bin/deploy
~/bin/deploy version
```

To verify a downloaded archive:

```bash
sha256sum -c checksums.txt --ignore-missing
```
