# The extension manifest

```yaml
version: "1.0"
title: File Browser
requirements:
  - which: python3
    homePage: https://www.python.org
rootItems:
  - title: Browse Root Directory
    command: browse-files
    with:
      root: /
  - title: Browse Home Directory
    command: browse-files
    with:
      root: "~"
  - title: Browse Custom Directory
    command: browse-files
    with:
      root:
        type: textfield
        title: Root Directory
  - title: Browse Current Directory
    command: browse-files
    with:
      root: "."
commands:
  browse-files:
    exec: ./file-browser.py --root ${{ root }}
    onSuccess: push-page
    params:
      - name: root
        type: directory
```
