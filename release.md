## Release Process

### Push

```
$ git push origin master
```

### Tag

```
$ git tag vX.X.X
$ git push --tags
```

Note:

- Replace `vX.X.X` with the target/new release version
- Do not use `git push --follow-tags` command as it may cause stuck release problem
