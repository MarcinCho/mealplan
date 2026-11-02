# Changelog

## Unreleased

### Nowy wygląd

- Ekran „Dziś" jako strona startowa: najbliższy niegotowy posiłek dnia, pasek dni tygodnia, lista posiłków z kucharzem i przycisk „Zrobione" (`plan_entry.done`, migracja 0003, pole `done` w API; kopiowanie tygodnia zeruje stan). Siatka tygodnia przeniesiona na `/plan`.
- Szybkie dodawanie posiłku z podpowiedziami przepisów, dniem, posiłkiem i kucharzem.
- Nawigacja: ciemny pasek boczny na komputerze, dolny pasek na telefonie; ustawienia (kolory, motyw, domownicy, wylogowanie) w menu awatara.
- Material 3: duże zaokrąglenia, płaskie powierzchnie z cienką ramką, dolny arkusz zamiast okna dialogowego na telefonie, puste stany z ilustracją, płynne przejścia.
- Palety kolorów (Klasyczny, Śliwka, Ziemia, Granat), domyślnie Ziemia.

### Etap 2: rdzeń

- Przepisy (składniki, kroki, tagi, `sourceUrl` tylko http/https), tygodniowy plan (wpis: przepis albo tekst wolny, slot, porcje, kucharz), kopiowanie tygodnia, lista zakupów generowana z planu z ręcznymi pozycjami, jednostki i normalizacja nazw.
- Zarządzanie członkami: lista, zmiana roli, usuwanie (kasuje sesje członka w gospodarstwie i czyści go jako kucharza); ostatni administrator chroniony (`409 last_admin`). Włączone klucze obce w SQLite.
- UI: widoki przepisów, planu tygodnia, listy zakupów i członków.
- Rozstrzygnięcia: przepis użyty w planie nie da się usunąć (`409 in_use`); kopiowanie tygodnia tylko na pusty tydzień (`409 conflict`); pozycje wygenerowane są tylko do odhaczania, edytować i usuwać można ręczne; regeneracja zachowuje pozycje ręczne oraz `id` i `checked` niezmienionych pozycji (zmiana ilości zeruje odhaczenie); generowanie z zakresem dni zastępuje wygenerowane pozycje całego tygodnia pozycjami z zakresu; tydzień identyfikuje poniedziałek `YYYY-MM-DD`; odejmowanie zapasów podpięte, ale nieaktywne do Etapu 3.

### Etap 1

- Etap 1: konfiguracja, baza z migracjami (SQLite, pod Postgres), logowanie argon2id, sesje, CSRF, limity prób, zaproszenia, API z OpenAPI, powłoka UI Vue + Vuetify, osadzony frontend, Dockerfile, CI.
- Poprawki po przeglądzie końcowym: sprawdzenie konfiguracji przed haszowaniem, ograniczenie współbieżności argon2, odrzucanie zbyt długich loginów, polskie komunikaty błędów i podpowiedź loginu, poprawki dokumentacji (zapraszanie, blokady, README) i CI. Znana, nieużywana luka GO-2026-5932 (`golang.org/x/crypto/openpgp`, brak poprawki, pakiet nie jest importowany).
- Toolchain Go 1.26 i aktualne zależności (m.in. pgx v5.12.0, modernc.org/sqlite v1.61.1, golang.org/x/crypto v0.58.0, golang.org/x/text v0.43.0); `govulncheck` nie zgłasza podatności w używanym kodzie.
