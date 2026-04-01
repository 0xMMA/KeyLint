# Raw Input
```
hallo Alex und Sam
kleines status update zur datenqualität
# angepasst / erledigt
v_overview_2 (base of v_overview_incl_follower)
- fix: Facebook organic reach angepasst (vorher NULL)
- fix: Aus irgendeinem Grund haben die daten von Instagram Organic Komplett gefehlt, diese sind jetzt ins view aufgenommen
# Neue/veränderte To-dos für mich (vollständigkeitshalber aufgenommen)
- [ ] #task Still open: Keine Follower auf Instagram nach August 2024 (siehe `follower_count`, fehlerhaft, wird bis dahin pausiert).
- [ ] #task new issue: issue  instagram follower_count ist fehlerhaft -> anfrage an adverity
# analysiert
	
- [x] #task #analyse Facebook hat keine Kommentare und Shares - 2h ⏫ ✅ 2025-04-07
	- behoben für organic
	- Keine Shares/Kommentare/Interaktionen bei Paid.
	- Paid hat aber `unique_actions`, vielleicht könnte man diese verwenden.
	- @ Alex bitte mach dir dazu gedanken, alternativ können wir auch mal gemeinsam auf die möglichen verfügbaren felder schauen
	
- [x] #task #analyse Instagram hat keine Shares und teilweise 0 Interaktionen trotz Likes and Kommentaren - 2h ⏫ ✅ 2025-04-07
	- behoben für organic
	- Keine Shares/Kommentare/Interaktionen bei Paid.
	- Paid Interaktionen gibt es nicht, es gäbe aber **unique_actions** und video completions (75% etc )
	- @ Alex bitte mach dir dazu gedanken, alternativ können wir auch mal gemeinsam auf die möglichen verfügbaren felder schauen
	
- [x] #task Keine Follower Zahlen für Instagram Organic ⏫ ✅ 2025-04-07
	- Kann einfach hinzugefügt werden (letztes vod) - 2h
	- semi erledigt follower_count ist "defekt" -> anfrage an adverity (siehe neues todo)
	
- [x] #task #analyse LinkedIn Organic keine Interaktionen trotz Likes, Shares und Kommentaren - 2h ⏫ ✅ 2025-04-07
	- fixed
	- 
- [ ] #task Story (campaign), Thema (cluster), Themenbereich (topic), Akteure (actor) sind alle konsistent leer - 8h ⏫
	- ~~Join in der view wohl falsch~~
	- Leider wurde hier im view garnichts gemacht, ich fürchte das könnte mehr aufwand werden, kann ich aber noch nicht sagen
	- nach wie vor offen, mehr analyse zeit nötig
```

# User accepted output
```
# Datenqualität Update | LinkedIn-Ausfall | Nate: Dringend Termin benötigt

Hallo Alex und Sam,

ich möchte euch über die aktuellen Entwicklungen bezüglich unserer Datenqualität informieren.

## 🟢 Bereits umgesetzt
- **v_overview_2** (Basis für v_overview_incl_follower):
  - Facebook Organic Reach korrigiert (vorher NULL)
  - Fehlende Instagram Organic Daten ins View integriert
- **Facebook (Organic)**: Kommentare und Shares behoben
- **Instagram (Organic)**: Shares behoben
- **LinkedIn Organic**: Interaktionsprobleme trotz Likes, Shares und Kommentaren behoben

## 🟡 Neue/aktuelle To-dos
- Instagram: Keine Follower-Daten nach August 2024 (`follower_count` fehlerhaft, wird pausiert)
- **NEU**: Instagram `follower_count` fehlerhaft → Anfrage an Adverity wird gestellt
- **NEU DRINGEND**: **LinkedIn Autorisierung komplett ausgefallen** für LinkedIn Organic und Paid → **@Nate**: Bitte melde dich, wann du heute Zeit hast
- **Metadaten-Felder**: Story (campaign), Thema (cluster), Themenbereich (topic), Akteure (actor) durchgehend leer - umfangreichere Analyse erforderlich
- **Bereinigte Interaktionen für Facebook**: 
  - Für Facebook Organic vorhanden
  - Für Facebook Paid auch vorhanden, aber häufig mit Wert 0.00
- **Beitragsformat-Problematik (newformat)**:
  - Nur Instagram hat Format mit CAROUSAL_ALBUM, IMAGE, VIDEO

## 🔍 Offene Punkte für Feedback
1. **Facebook Paid**:
   - Keine Shares/Kommentare/Interaktionen, aber `unique_actions` verfügbar
   - **@Alex**: Bitte evaluieren oder gemeinsam verfügbare Felder prüfen

2. **Instagram Paid**:
   - Keine Shares/Kommentare/Interaktionen, aber `unique_actions` und Video-Completions (75% etc.) verfügbar
   - **@Alex**: Bitte evaluieren oder gemeinsam verfügbare Felder prüfen

Viele Grüße
``` 