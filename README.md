# dots

Command line tool for managing dots files and computer configuration.

## Install

```sh
make install
```

## Uninstall

```sh
make uninstall
```

## TODO

- [ ] `update` and `sync` are not the best abstractions, consider changing them.
- [ ] BUG: The `update` command doesn't allow you to update one file. For
    example, `dots update ~/.bashrc` will update all changes to all the tracked
    files not just the file given as an argument.
- [ ] Add encryption/decryption.
