## Co i dlaczego

<!-- Krótki opis zmiany i powiązane issue (np. Closes #123). -->

## Lista kontrolna

- [ ] Testy Go (`go test ./... -race`) i frontendu (`npm test`, `npm run build`) przechodzą
- [ ] `gofmt -l .` jest puste, `go vet ./...` przechodzi
- [ ] `api/openapi.yaml` zaktualizowany (jeśli zmieniło się API)
- [ ] Dokumentacja w `docs/` zaktualizowana (jeśli trzeba)
- [ ] Wpis w `CHANGELOG.md`
- [ ] Migracje (jeśli są) działają na SQLite i Postgresie; uwagi o migracji opisane poniżej
- [ ] Zrzuty ekranu dla zmian w interfejsie

## Uwagi o migracji / wdrożeniu

<!-- Np. nowe zmienne środowiskowe, kroki aktualizacji. Wpisz "brak", jeśli nie dotyczy. -->
