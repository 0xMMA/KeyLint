# Raw Input
```
Grüße zusammen,
- ich wurde heute zu einem termin eigenladen in der Contoso
- das ergebnis des Termins ist, das die Contoso sich entschieden hat zu Airbyte zu wechsel
	- Situation: es gibt 3 Stakeholder für den bedarf "Marketing Insights aus Social Media herausziehen". Bisher war unsere Lösung mit Adverity nur in einer Abteilung (mit fehlt für abteilung hier der korrekte begriff, daher ersatzweise Abteilung gwählt) im einsatz
	- Grund 1 : keine silo lösungen pro abteilung gewünscht
	- Grund 2: Aufgrund der cross finanzierung über 3 Abteilungen, ist die airbyte variante deutlich günstiger für Alex' Abteilung	
	- Weitere Faktoren:  
		- Mann will hier Vorteile von Google BigQuery und Als dashboard lookerstudio (scheinbar kostenlos) von google nutzen
		- Mann will die Datenbanken in das Bank interne Netz holen, um auch andere Resourcen mit dem ETL Funktionen von Airbyte anzubinden	
	
- Weitere Informationen
	- die Vorgeschlagene Lösung mit Airbyte als zentrales tool wurde duch einen Externen mit Aufwandsabschätzung (Quasi Angebot) bediehnt
	- Die beauftragung des externen Jordan Fischer (**nicht** Northwind Berater) erfolgte vermutlich aus den Anderen beiden Contoso abteilungen
		- wir sind jetzt hier quasi in einer Position das unsere Lösung für Alex wegoptimiert wird
		- ich sehe keine chance diese entscheidung (wechsel von adverity zu airbyte) umzustoßen, die Freigabe wurde heute quasi im Termin erteilt
		- ich sehe dennoch eine chance ein parallel angebot für die Umsetzung aufzustellen,
			- bei dem wir als Northwind die zentrale / airbyte lösung anbieten 
			- Evtl eine andere Form der Datenbank hosting für das interne Contoso netz anbieten (ich kenne hier leider den aufbau nicht, es hat mcih schon verwundert das man Google Bigquery in der Cloud als interne Lösung bezeichnet)
				- Ich vermute wir als Northwind können einen ähnlichen aufbau anbieten oder alternativ in AWS umsetzen
			- Beim Dashboarding das kostenlose lookerstudio von google wird nicht ganz einfach
				- evtl könnte man hier die vorhandenen von uns entwickelten Qlik Dashboards bewerben, eine Alternative finden oder ebenfalls Lookerstudio Dashboards anbieten
	- Ich wude bereits beauftrag für den anderen externen Berater eine aufstellung der KPIs zu erstellen (in form von schemata exports)
	- Mir wurde auch eine abschätzung abverlangt für den wissens & datentransfer zu besagtem Berater (inklusive Postgres dump + hinweise welche transformationen  die Ruby app vornimmt) den umfang hierfür habe ich zunächst auf 5PT geschätzt
- Ich bin selbst kein Vertriebler und in das Projekt aufgenommen um ein Legacy projekt weiterzupflegen und sehe es nicht in meiner verwantwortung bzw als meine aufgabe zu entscheiden ob wir hier ein gegen bzw parallel angebot in den raum werfen oder nicht, geschweige dieses auszuarbeiten
- Es bedarf hier dringend der sichtung und übernahme durch einen höheren Projekt verantwortlichen bzw einen Vertriebler
```
# User accepted output
```
Entscheidung Contoso: Wechsel zu Airbyte | Auftrag droht an externe Beratung zu gehen | Benötige Vertriebliche Unterstützung

Liebe Kolleginnen und Kollegen,

heute wurde ich zu einem Termin bei der Contoso Bank eingeladen, dessen Ergebnis ich euch hiermit mitteilen möchte.

## Kernergebnis
Die Contoso Bank hat beschlossen, von Adverity zu Airbyte zu wechseln, um Marketing-Insights aus Social Media zu gewinnen. Diese Entscheidung wurde heute praktisch finalisiert. **Der Auftrag wird vermutlich an eine andere Beratung (Jordan Fischer) vergeben, wenn wir nicht aktiv gegensteuern.**

## Hintergründe der Entscheidung
- **Stakeholder-Situation**: Es gibt drei Abteilungen mit Bedarf an Social Media Marketing-Insights. Bisher war unsere Adverity-Lösung nur in einer Abteilung im Einsatz.
- **Hauptgründe**:
  1. Keine Silo-Lösungen pro Abteilung gewünscht
  2. Durch Cross-Finanzierung über drei Abteilungen ist die Airbyte-Variante für Alex' Abteilung deutlich kostengünstiger

- **Weitere Faktoren**:
  - Nutzung von Google BigQuery und dem kostenlosen Looker Studio als Dashboard-Lösung
  - Integration der Datenbanken ins bankinterne Netz, um weitere Ressourcen mit Airbyte-ETL-Funktionen anzubinden

## Zusätzliche Informationen
- Die vorgeschlagene Airbyte-Lösung wurde von einem externen Berater (Jordan Fischer, kein Northwind-Berater) mit Aufwandsabschätzung präsentiert
- Die Beauftragung des Externen erfolgte vermutlich durch die anderen beiden Contoso-Abteilungen
- Unsere bisherige Lösung für Alex wird faktisch wegoptimiert

## Chancen für Northwind
Obwohl die Grundsatzentscheidung für Airbyte gefallen ist, sehe ich folgende Möglichkeiten:
- Ein Parallelangebot für die Umsetzung aufstellen, bei dem Northwind die zentrale Airbyte-Lösung anbietet
- Alternative Datenbankhosting-Optionen für das interne Contoso-Netz vorschlagen (AWS-basiert)
- Beim Dashboarding unsere vorhandenen Qlik-Dashboards bewerben oder Looker Studio-Alternativen anbieten

## Aktuelle Aufgaben
- Ich wurde bereits beauftragt, für den externen Berater eine Aufstellung der KPIs zu erstellen
- Mir wurde auch eine Abschätzung für den Wissens- und Datentransfer abverlangt (inklusive Postgres-Dump und Transformationshinweise) – Umfang ca. 5 PT

## Benötige dringend Unterstützung
Als technischer Mitarbeiter ohne Vertriebsfunktion sehe ich es nicht als meine Aufgabe, über ein Gegenangebot zu entscheiden oder dieses auszuarbeiten. Es bedarf hier dringend der Übernahme durch einen Projektverantwortlichen oder Vertriebsmitarbeiter.

Bitte um zeitnahe Rückmeldung.

Mit freundlichen Grüßen
```