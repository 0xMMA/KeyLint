# Raw input
```

Grüße,
Wir haben uns entschieden, eine minimalistische Diagnostics-Funktion über ein Release/Update bereitzustellen. Nach Rücksprache mit Ethan werden wir am Montag gegen 14 Uhr dieses Diagnose-Update in die FinReg deployen. das update wird aus erweiterten logeinträgen bestehn, sowie der möglichkeit consolen logs direkt in der extension einzusehen. Wir erhoffen uns davon anschließend weitere erkenntnisse bezúglich der zuvor erwáhnten und verwendeten ${window.location.origin} und fehlern die vom embedded webview evtl verschluckt werden.

Mfg
Michael

---- Mail historie: 
 **Betreff:** Notwendigkeit von Logs aus der DataBridge Extension
 
Hallo Eth, hallo Michael,
wir stehen aktuell vor dem Problem, dass sich die Extension unter Tableau Desktop (Citrix) nicht wie erwartet verbindet und lediglich eine weiße Seite angezeigt wird. Citrix-seitig wurde bereits alles Mögliche geprüft und ausgeschlossen – es deutet nun vieles darauf hin, dass das Problem direkt mit der Extension zusammenhängt.
Die Schwierigkeit dabei: Die relevanten Logs landen offenbar nicht im Backend, sondern sind nur über die Browserkonsole einsehbar – was in Tableau Desktop nicht ohne Weiteres möglich ist. Genau diese Browserlogs würden uns im aktuellen Fall aber am meisten weiterhelfen.
Daher unsere Bitte: Gibt es eine Möglichkeit, in irgendeiner Form (z. B. durch Weiterleitung ins Backend) Logs aus der Web-App/Extension zu erhalten, die in Tableau Desktop läuft? Ohne diese Einsicht bleiben uns die Hände gebunden, was die Fehlersuche betrifft.
Danke euch und viele Grüße
Leo
```

# user accpted output
```
**Betreff:** Diagnose-Update für DataBridge Extension | FinReg Deployment Montag 14 Uhr | Erweiterte Logs für Tableau Desktop Citrix Problem | Fehlerbehebung ${window.location.origin} und Webview-Fehler

Hallo,

**Diagnose-Update wird Montag 14 Uhr in FinReg deployed**
- Minimalistische Diagnostics-Funktion über Release/Update
- Nach Rücksprache mit Ethan terminiert
- Deployment-Zeitpunkt: Montag gegen 14 Uhr

**Konsolen-Logs werden direkt in Extension verfügbar**
- Erweiterte Logeinträge im System
- Konsolen-Logs direkt in Extension einsehbar
- Keine Backend-Weiterleitung erforderlich

**Durch Update wird mit Erkenntnisgewinn zum Fehler "Weiße Seite" gerechnet**
- Weitere Erkenntnisse zum Fehler "Weiße Seite in Tableau Desktop (Citrix)"
- Aufdeckung von im Webview verschluckten Fehlern
- Verifikation der ${window.location.origin} als Fehlerverursacher

Viele Grüße
Michael
```