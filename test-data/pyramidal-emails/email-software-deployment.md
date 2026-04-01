# Raw input
```
Hallo Ethan
- im folgenden hier der Status DataBridge mit Hinsicht auf das Deployment, wie gewünscht
- Wir haben jetzt für die DataBridge Extension ein [README.md - Repos](https://dev.azure.com/northwinddev/_git/Tableau%20Enterprise%20Tools?path=/Northwind.TableauTools.TableauExtensions/DataBridge/README.md&_a=preview)
- Ich habe heute DataBridge 1.0, und DataBridge 1.1 auf dem Tableau Demo Server lauffähig gesehen, und getestet
	- der demo server ist allerdings nach wie vor aufgrund von Performance in keinster weiße geeignet für Präsentationen gegenüber dem Kunden 
- Da DataBridge 1.2 reines Logging ist, kann man davon nicht viel sehen
- aus meiner sicht steht einem deployment in der FinReg nichts im weg
- Wir sollten allerdings genug zeit einplanen um im Demo System der FinReg - die 1.0 und 1.1 durchzukonfigurieren, sodass diese Präsentabel sind. ich schätze hier mindestens 1 Tag aufwand
- In der Readme siehe oben sind auch Known Issues gelistet die wir vermutlich vor einem endgültigen Abschluss noch erledigen bze. addressieren müssen.
	- ## Known Issues[](https://dev.azure.com/northwinddev/_git/Tableau%20Enterprise%20Tools?path=/Northwind.TableauTools.TableauExtensions/DataBridge/README.md&_a=preview&anchor=known-issues)
		- Since the design config does not include information about the column's properties like autoIncrement, isUnique, etc., the export table schema does neither. For use case 1.1, an auto-incremented primary key appears to be required. Right now, the JSON for creating both design and live tables needs to be manually adjusted.
		- Audit Logging does only work for tables created through the Extension or Tabtools.
		- When the connection to the Tableau Server is lost or interrupted, reconnecting does not restore the connection to the extension dialog, which will appear to be stuck in a loading state. The dialog will need to be closed and reopened.
		- The tableau extension.js file and the deveextreme-licence.js file are way too large.
		- Since the tableau column IDs are unreliable, the worksheet column names were used. These need to be updated after making changes to the tableau column and may differ for user with english and german language settings.
		  
  Viele Grüße
  Michael
```
# User accepted output
```
# DataBridge Status & Deployment | Konfigurationsbedarf für FinReg | Freigabe erforderlich

Hallo Ethan,

ich möchte dich über den aktuellen Status des DataBridge-Deployments informieren:

**Fortschritte:**
- Die vollständige Dokumentation für die DataBridge Extension ist nun in der [README.md](https://dev.azure.com/northwinddev/_git/Tableau%20Enterprise%20Tools?path=/Northwind.TableauTools.TableauExtensions/DataBridge/README.md&_a=preview) verfügbar
- DataBridge 1.0 und 1.1 wurden erfolgreich auf dem Tableau Demo Server getestet und sind funktionsfähig
- DataBridge 1.2 (reines Logging) ist implementiert

**Deployment-Status:**
- Aus meiner Sicht kann das Deployment bei der FinReg durchgeführt werden
- Der aktuelle Demo-Server ist jedoch aufgrund von Performance-Problemen nicht für Kundenpräsentationen geeignet

**Nächste Schritte:**
- Wir sollten mind. einen Tag für die Konfiguration von DataBridge 1.0 und 1.1 im FinReg-Demosystem einplanen, um eine präsentable Umgebung zu schaffen
- Vor Projektabschluss müssen wir noch die bekannten Probleme adressieren:
  - Fehlende Spalteneigenschaften (autoIncrement, isUnique) im Design-Config
  - Audit-Logging funktioniert nur für Tabellen, die über Extension/Tabtools erstellt wurden
  - Verbindungsprobleme bei Unterbrechungen zum Tableau Server
  - Optimierungsbedarf bei Dateigröße (extension.js, deveextreme-licence.js)
  - Probleme mit Spaltennamen bei verschiedenen Spracheinstellungen

Bitte gib mir Feedback, ob wir mit dem Deployment fortfahren können und ob du Unterstützung bei der Konfiguration benötigst.

Viele Grüße
Michael
```