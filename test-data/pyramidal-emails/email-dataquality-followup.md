# Raw Input
```

Hallo Alex und Sam
- LinkedIn Authorisierung  - funktioniert wieder.
- Story (campaign), Thema (cluster), Themenbereich (topic), Akteure (actor)  - funktionieren jetzt
- Problematik mit Beitragsformat (newformat) - ist behoben
	- Hier wurde für Instagram ein Feld verwendet, welches aus Adverity kommt, ohne dieses gegen unsere **newformats**-Tabelle zu matchen. Ich denke, es wäre eventuell möglich, für Instagram zu versuchen, einen Automatismus für das Feld **newformat** zu bauen (sodass dies nicht manuell im Tagging-Tool gemacht werden muss oder vorselektiert ist). Ich betrachte dies jedoch als extra Aufgabe/Aufwand, sofern überhaupt gewünscht. @Alex dazu wäre eine rückmeldung gut
- Keine Interaktionen für Facebook Paid  - ist behoben
- Gestern habe ich einige Änderungen in einem **View** gebaut, welches jedoch leider gar nicht in **Qlik** verwendet wurde. Die Änderungen sollten jetzt aber auf das richtige view übertragen sein.
- SMTQM-42 Reichweite - sieht für mich funktional aus 
- instagram follower_count ist nach wie vor fehlerhaft
- aus meiner sicht ist dann bis auf den follower_count die Themen "Datenqualität" die per Mail (Derek: "AW: Status Quo") kommuniziert wurden erledigt
- als nächsten schwerpunk/ focusthema gehe ich das ticket **SMTQM-15** an
- ich werde den rest der woche vorraussichtlich primär in meinem Hauptprojekt tätig sein müssen, ich würde euch daher bitten, die anpassungen frühzeitig zu evaluieren und zu schauen ob noch sehr dringende DatenQualitäts themen offen sind, damit ich diese optimal für die kommende woche priorisieren kann.
  
Bei allen genannten Themen würde ich mich über Feedback und Validierung freuen. Mehr Augen sehen mehr.
```

# User accepted output
```
# Datenqualitäts-Meilenstein erreicht | (fast alle?) Probleme behoben | Rückmeldung gewünscht

Hallo Alex und Sam,

ich habe die aktuellen Fortschritte bei unseren Datenqualitäts-Themen strukturiert zusammengefasst:

## ✅ Behobene Probleme
- **LinkedIn Autorisierung**: Funktioniert wieder vollständig
- **Modulare Tracking-Komponenten**: Story (campaign), Thema (cluster), Themenbereich (topic) und Akteure (actor) arbeiten jetzt korrekt
- **Beitragsformat-Problem (newformat)**: Fehler wurde behoben
- **Facebook Paid Interaktionen**: Fehlerhafte Daten wurden korrigiert
- **SMTQM-42 (Reichweite)**: Erscheint funktional aus meiner Sicht

## ⚠️ Offene Themen
- **Instagram follower_count**: Enthält nach wie vor fehlerhafte Daten

## 🛠️ Wichtig: Weitergabe an Sam
**Alle per Mail kommunizierten Datenqualitäts-Themen sind jetzt erledigt** (mit Ausnahme des follower_count). Die Daten sollten für die Weiterarbeit bereit sein.

## 🔍 Details zum Beitragsformat
Für Instagram wurde ein Feld aus Adverity verwendet, ohne es gegen unsere **newformats**-Tabelle abzugleichen. Eventuell könnten wir einen Automatismus für das Feld **newformat** entwickeln (um manuelle Eingaben im Tagging-Tool zu vermeiden).  
**@Alex**: Deine Rückmeldung hierzu wäre wichtig - sollen wir diesen Zusatzaufwand einplanen?

## 📋 Nächste Schritte
- Werde als nächstes Ticket **SMTQM-15** in Angriff nehmen
- Bin für den Rest dieser Woche größtenteils in meinem Hauptprojekt gebunden
- **Bitte um zeitnahe Prüfung** der umgesetzten Anpassungen und Mitteilung eventueller dringender Datenqualitäts-Themen, damit ich diese optimal für kommende Woche einplanen kann

Ich freue mich über euer Feedback und eure Validierung zu allen genannten Punkten. Mehr Augen sehen mehr!

Viele Grüße
```