language: de
language-anchors: KeyLint, Import, Schnittstelle, State
forbidden: interface, Zustand
must-change: daten->Daten; wartungsfenster->Wartungsfenster; import->Import
tone-anchors: bitte nicht vorher anfassen
min-length-ratio: 0.9
max-length-ratio: 1.25
why: A product name must keep its own casing, and "Schnittstelle" is already
  German — a model that anglicises it has edited in the wrong direction.
  Deliberately avoids the word "release": the prompt's own fifth example already
  corrects "das release" to "das Release", and a sample that repeats it would
  measure recall of that example rather than the rule behind it.
