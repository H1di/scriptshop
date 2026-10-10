# Scripts

How to put a script on the site, change it, and take it down. All commands run on the server, using the [`shop` shortcut](install.md#step-8-set-up-the-shop-shortcut).

## What a script has

| Field | Example | Notes |
|-------|---------|-------|
| ID | `001` | Given automatically. Never reused, even after a script is deleted |
| Name | `backup.sh` | Shown in the catalog and used as the download file name. Defaults to the file name |
| Version | `1.2` | Optional, any text |
| Price | `$40` | Optional, any text: `$40`, `€35`, `on request` |
| Description | `Incremental backup with rotation.` | Optional. One or two sentences |
| File | | Up to 100 MB. Stored under a random name; the original file is not needed afterwards |

## Put a new script on the site

**1. Copy the file to the server.** From your own computer:

```sh
scp backup.sh you@shop.example.com:/tmp/
```

**2. Make it readable for the site.** On the server:

```sh
chmod 644 /tmp/backup.sh
```

The `scriptshop` user must be able to read the file. Files in your home folder usually are not readable for it, which is why `/tmp` is used.

**3. Add it.** Options go **before** the file name:

```sh
shop script add -version 1.2 -price '$40' -desc 'Incremental backup with rotation.' /tmp/backup.sh
```

It prints the new ID:

```
added 001 backup.sh
```

Use single quotes around a price with `$`, otherwise the shell swallows it.

**4. Delete the copy.**

```sh
rm /tmp/backup.sh
```

**5. Check it.** Log in to the site with your own login. The script is in the catalog with its price. Nobody can download it yet: access is given per user, see [Users → Give access to scripts](users.md#give-access-to-scripts).

To see it the way a paying client does, grant it to yourself:

```sh
shop grant owner 1
```

Now `[download]` appears instead of the price. Click it and check the file.

## See all scripts

```sh
shop script list
```

```
ID   NAME        VERSION  PRICE  SIZE  SHA256
001  backup.sh   1.2      $40    22B   c0e4bd8e3688470e
002  netscan.py  0.9      $120   9B    cc42155088fca573
```

`SHA256` is the start of the file's fingerprint. It changes when the file changes, so you can confirm an upload went through.

## Change price, name, version or description

Give the ID, then only the fields you want to change. Everything else stays as it is.

```sh
shop script edit 1 -price '$50'
shop script edit 1 -name 'backup-pro.sh' -desc 'Incremental backup with rotation and checksums.'
```

The site shows the change within a second. Clients just reload the page.

## Release a new version of a script

Upload the new file as in steps 1 and 2 above, then:

```sh
shop script edit 1 -version 1.3 -file /tmp/backup.sh
rm /tmp/backup.sh
```

- The ID stays the same, so everyone who had access keeps it and gets the new file.
- The old file is deleted from the server.
- Check `shop script list`: `SIZE` and `SHA256` should change.

## Take a script off the site

```sh
shop script del 1
```

This removes the script from the catalog, deletes its file, and removes it from every user's grants. It cannot be undone, except from a [backup](maintenance.md#back-up).

To stop selling a script but let existing owners keep downloading it, do not delete it. Mark it instead and stop granting it:

```sh
shop script edit 1 -price 'not sold'
```

## Quick reference

| Task | Command |
|------|---------|
| Add | `shop script add -version V -price 'P' -desc 'D' /tmp/file` |
| List | `shop script list` |
| Change fields | `shop script edit <id> -price 'P'` (any of `-name -version -price -desc`) |
| Replace the file | `shop script edit <id> -file /tmp/file` |
| Delete | `shop script del <id>` |

Next: [create users](users.md).
