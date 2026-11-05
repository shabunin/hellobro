# Dictionaries

The reader looks a held word up (see [usage](usage.md), Books) in the
StarDict dictionaries in `Dictionaries/` on the Kobo's storage: any
you have, or ones made of Wiktionary's words, as kaikki.org gives them.
The meanings of those are in English (they're English Wiktionary's); the
text is CC BY-SA.

## Making one

On the PC, as the files are big:

```sh
curl -o grc.jsonl 'https://kaikki.org/dictionary/Ancient%20Greek/kaikki.org-dictionary-AncientGreek.jsonl'
go run ./cmd/dictconv -name 'Ancient Greek – English' grc.jsonl out/grc-en
```

The file's name starts with its language, as a book's `dc:language` says
it (`grc`, `el`, `fr`, `ru`), and those come first for such a book. All
are asked anyway, as books come in two languages, or say the wrong one.

## Putting it on the Kobo

Copy all of its files in (`.hbx` too) over file sharing, into
`Dictionaries/`:

```sh
ssh -p 2022 kobo.local mkdir -p /mnt/onboard/Dictionaries
scp -P 2022 out/grc-en.* kobo.local:/mnt/onboard/Dictionaries/
```

Replacing one that's been used wants a restart: it's kept open.

## How a word's found

Folded (`dict.Fold`): lower case, no accents, no other marks, one
apostrophe and one sigma. A book's Greek changes its accents as it goes
(`καλὸς`, `καλός`) and Wiktionary marks vowels' length (`σκῠ́λος`). Words
that are one when folded (`мой`, `мои`) are all shown, the one written as
in the book first. Then a word's other forms, in the `.syn`.

The folded words are kept beside the dictionary, sorted, in a `.hbx`, and
searched there on disk: no load, no memory. `dictconv` writes it. A
dictionary without one gets it at its first look up, which takes a while
on the Kobo for a big one (the box says it's looking), and only once.
