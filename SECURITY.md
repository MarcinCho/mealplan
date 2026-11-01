# Polityka bezpieczeństwa

## Wspierane wersje

Projekt jest przed wersją 1.0. Poprawki bezpieczeństwa trafiają tylko do najnowszej wersji (aktualny stan gałęzi głównej). Starsze wydania nie dostają łatek.

## Jak zgłosić podatność

Nie zgłaszaj podatności w publicznym issue. Użyj prywatnego zgłoszenia GitHub (Security advisories):

https://github.com/MarcinCho/mealplan/security/advisories/new

(Na stronie repozytorium: Security, Report a vulnerability.)

Jeśli to możliwe, podaj:

- opis problemu i jego skutek (co może zrobić atakujący),
- kroki do odtworzenia lub krótki dowód koncepcji,
- wersję lub commit oraz sposób wdrożenia (Docker, binarka, sterownik bazy),
- ewentualną propozycję poprawki.

## Czego się spodziewać

Mealplan to projekt hobbystyczny, utrzymywany po godzinach, więc czasy są orientacyjne i nie stanowią zobowiązania:

- potwierdzenie zgłoszenia: zwykle w ciągu kilku dni,
- ocena i plan poprawki: zwykle w ciągu 2 tygodni,
- poprawka: zależnie od wagi problemu, najpierw te krytyczne.

## Ujawnianie

Prosimy o skoordynowane ujawnianie: nie publikuj szczegółów, dopóki poprawka nie jest dostępna lub nie ustalimy wspólnie terminu. Po wydaniu poprawki opublikujemy poradnik bezpieczeństwa (GitHub Security Advisory) i wymienimy zgłaszającego, o ile sobie tego życzy.

## Model zagrożeń

Zaimplementowane zabezpieczenia i znane ograniczenia opisuje [docs/security.md](docs/security.md).
