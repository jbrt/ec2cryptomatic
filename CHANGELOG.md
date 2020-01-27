# Changelog

## 1.1.2

- Fixing a problem with AWS tags (can't copy AWS tags) 

## 1.1.1

- Fixing a problem with BOTO waiters (adding a maximum retry value) 

## 1.1.0

- Upgrade Boto modules
- Changing old-style Python strings to new Python 3.6 fstrings
- Simplifying encryption algorithm (since AWS let create encrypted volume from snapshot directly)
- Fixing a bug on IO1 volume encryption