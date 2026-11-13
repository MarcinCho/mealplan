# Mealplan

[![ci](https://github.com/MarcinCho/mealplan/actions/workflows/ci.yml/badge.svg)](https://github.com/MarcinCho/mealplan/actions/workflows/ci.yml)
[![Licencja: MIT](https://img.shields.io/badge/licencja-MIT-blue.svg)](LICENSE)

Lekka domowa aplikacja do planowania jadłospisu na tydzień: przepisy, kto gotuje które danie, lista zakupów, która pamięta, co masz w domu, gotowanie na kilka dni naraz i kalorie. Działa w przeglądarce na telefonie i komputerze, na własnym serwerze (selfhost), z bazą SQLite w jednym pliku (albo w Postgresie).

## Funkcje

- **Przepisy:** składniki, kroki, porcje i tagi. Import z linku do strony albo z wklejonego tekstu. Umiejętność dla Claude zamienia dowolny przepis na format do wklejenia, od razu z wyliczonymi kaloriami.
- **Plan tygodnia:** kto gotuje które danie i na ile porcji. Ekran „Dziś” pokazuje posiłki dnia.
- **Lista zakupów:** generowana z planu. Odejmuje to, co masz w spiżarni, a kupione rzeczy jednym kliknięciem przenosi do spiżarni.
- **Spiżarnia:** ilości, daty ważności, stały zapas i comiesięczny przegląd. „Zrobione” przy posiłku zdejmuje składniki ze spiżarni.
- **Meal prep:** sesje gotowania wielu dań naraz, rozdzielanie porcji na dni i zamrażalka.
- **Wartości odżywcze:** baza produktów z kaloriami i makro, dzienny cel dla każdego domownika, dziennik dopisanych przekąsek.
- **Asystent produktów (opcjonalny):** z kluczem OpenRouter podpowiada produkty dla składników, znajduje duplikaty w bazie produktów i pomaga je scalić. Niczego nie zapisuje bez zatwierdzenia.
- **Domownicy:** konta, zaproszenia (link ważny 72 godziny) i role.
- **Języki:** polski i angielski, przełączane na stronie logowania i na stronie Konto.

## Szybki start

Pełny przewodnik krok po kroku (Docker, Docker Compose, kod źródłowy, HTTPS przez Tailscale, kopie zapasowe): [Szybki start](docs/quickstart.md).

```bash
docker build -t mealplan .
docker run -d --name mealplan --restart unless-stopped -p 127.0.0.1:8080:8080 -v mealplan-data:/data -e MEALPLAN_COOKIE_SECURE=false mealplan
# otwórz http://localhost:8080 i utwórz konto administratora
```

`MEALPLAN_COOKIE_SECURE=false` służy tylko do testu po zwykłym HTTP. Na co dzień użyj HTTPS i usuń tę zmienną.

## Dokumentacja

Instalacja i działanie:
- [Szybki start (selfhost)](docs/quickstart.md)
- [Wdrożenie selfhost z Tailscale](docs/deployment-selfhost.md)
- [Tryby i konfiguracja](docs/modes.md)
- [Bezpieczeństwo](docs/security.md)

Funkcje:
- [Import przepisów i umiejętność Claude](docs/import.md)
- [Wartości odżywcze, baza produktów i asystent](docs/nutrition.md)
- [Spiżarnia](docs/pantry.md)
- [Meal prep i zamrażalka](docs/mealprep.md)

Dla programistów:
- [Architektura](docs/architecture.md)
- [API](docs/api.md)
- [Współpraca](CONTRIBUTING.md)

## Projekt

- [Kodeks postępowania](CODE_OF_CONDUCT.md)
- [Polityka bezpieczeństwa](SECURITY.md)
- [Lista zmian](CHANGELOG.md)

## Licencja

MIT, zobacz [LICENSE](LICENSE).
