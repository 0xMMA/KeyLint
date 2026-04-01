# Raw Input
```
Im nachfolgenden die erste mail auf die ich antworte:
Hi Michael,
 
anbei meine Rückmeldung:
 
1. Meintest Du in der Auflistung der behobenen Probleme ggf. https://team-contoso.atlassian.net/jira/software/projects/SMTQM/boards/4?selectedIssue=SMTQM-43 (Unterticket von SMTQM-15) an Stelle von SMTQM-42?
 
1. Bitte erläutere mir außerdem noch, was sich hinter „newformats“ verbirgt.
 
1. Würdest Du mir die untenstehende Liste bitte nochmal aktualisieren?
 
1. Könntest Du mir bitte einen zeitlichen Forecast in der Erstellung der Auflistung für Jordan Fischer (im Termin besprochen) geben?
 
Danke vorab! 😊
 
___
 
**1. Verfügbarkeit "Aufrufe" (Views) via Schnittstellen**
**Facebook:**
·       [+] Für Videos verfügbar
·       [-] Für allgemeine Posts nicht verfügbar
**Instagram Organic:**
·       [+] Vollständig verfügbar
**Instagram & Facebook Paid:**
·       [-] "Aufrufe" direkt nicht verfügbar
·       [+] Alternative Metriken vorhanden:
o   "full_view_impressions": Anzahl vollständiger Aufrufe von Page-Posts durch Anzeigen
o   "full_view_reach": Anzahl Personen mit vollständigem Aufruf von Page-Posts durch Anzeigen
**2. Verfügbarkeit "Reichweite" in DB und QS**
**Mit Reichweite-KPI verfügbar:**
* [+] Instagram (Paid & Organic)
* [+] Facebook Paid
**Ohne Reichweite-KPI (nur Impressionen):**
·       [-] Facebook Organic
·       [-] LinkedIn (Paid & Organic)
**3. Aktionspunkt: Ergänzung "Aufrufe" Datentabelle in QS**
* [>] **To-Do (neu):** "Aufrufe" für Instagram Organic als zusätzliche Spalte aufnehmen
   * Beginne mit Priorität (Dringend) / Bitte Rückmeldung falls anders gewünscht @Carter, Alex
* [!] **Hinweis:** Für andere Kanäle nicht möglich, da KPI nicht verfügbar
**4. Status organische Impressionen in DB**
**Aktuelle Daten vorhanden:**
* [+] Facebook Organic & Paid: tagesaktuell
* [+] Instagram Paid: tagesaktuell
**Problem:**
·       [!] Instagram Organic: nur bis 10.03.2025 vorhanden
**Wichtig - Abkündigung durch Instagram:**
·       [!!] Folgenden Metriken werden am 21. April 2025 eingestellt:
o   `plays`
o   `clips_replays_count`
o   `ig_reels_aggregated_all_plays_count`
o   `impressions`
 
Liebe Grüße
Alex

----
hier die zweite mail auf die ich antworte
Hi Michael,
 
hier ein Feedback zu den untenstehenden Kennzahlen:
 
* Was verbirgt sich denn konkret hinter den Unique Actions? Bitte evaluiere anhand von folgender Beispielkampagne, welche Werte ausgegeben werden. Hilft Dir das, um auf die dort hinterlegten Zahlen zu schließen?
 
Erst dann kann ich eine Entscheidung treffen. Danke vorab!
 
Vielen Dank und viele Grüße
Alex
 ----

hier mein grob vorgeschriebenen informationen zu den punkten
Hallo Alex
-  Aufrufe (Views) ist jetzt hinzugefügt, ist aber nur für instagram organics gefüllt (die daten sind recht weit rückwärts verfügbar bis ca mitte letztes jahr)
- newformat bezieht sich auf "Problematik mit Beitragsformat (newformat) besteht weiterhin (Nur Instagram hat Format mit CAROUSAL_ALBUM, IMAGE, VIDEO) "
	- schlicht gesagt ist die spalte newformat jetzt funktional (wird gespeist aus eingaben aus dem tagging tool)
- `unique_actions` aufzudröseln/erklären schaffe ich vermutlich frühstens morgen
- Reichweite (reach) ist für alle verfügbar, aber bei FB Organic und Linked In Paid/Organic setzt sich das ganze aus impressions (unique) zusammen
	- ich glaube SMTQM-43 und SMTQM-42 sollten damit beide erledigt sein oder sehe ich das falsch ?
- die kpi liste für Jordan komme ich auch erst morgen zu

versuch aus meinen vorgeschriebenen infos keine informationen dabei zu verlieren
```

# User accepted output
```
# Rückmeldung auf Analyseberichte | Statusupdates zu KPIs | Korrektur zu newformat | Zeitliche Planung

Hallo Alex,

vielen Dank für deine Nachfragen. Hier meine Rückmeldungen zu allen Punkten:

## Status Views & Reichweite
- **Views/Aufrufe:** ✅ Für Instagram Organic hinzugefügt und rückwirkend bis Mitte letzten Jahres verfügbar
- **Reichweite (Reach):** ✅ Für alle Kanäle verfügbar
  - Bei FB Organic und LinkedIn (Paid/Organic) basierend auf unique impressions

## Begriffserklärungen & Tickets
- **"newformat":** Die Spalte ist jetzt funktional und wird direkt mit den Werten aus dem Tagging-Tool gespeist
  - Instagram verwendet nicht mehr CAROUSAL_ALBUM, IMAGE, VIDEO, sondern entspricht nun den anderen Kanälen mit den im Tagging-Tool ausgewählten Formaten
- **Tickets:** Sowohl SMTQM-43 (Reichweite-KPI für Instagram und Facebook) als auch SMTQM-42 sollten damit erledigt sein, da die Reichweite nun für alle Kanäle verfügbar ist

## Zeitplanung
- **Unique Actions Analyse:** Frühestens morgen möglich, benötige Zeit für Auswertung der Beispielkampagne
- **KPI-Liste für Jordan:** Ebenfalls erst morgen zu erwarten

Bitte teile mir mit, ob die Timeline so in Ordnung ist oder ob einzelne Punkte priorisiert werden sollten.

Viele Grüße
Michael
```