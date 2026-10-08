# Operação

Configure as variáveis conforme `specs/001-encurtador-analytics/contracts/config.md`. Guarde backup
do PostgreSQL e preserve `PEPPER` e `REPORT_TZ`: trocar qualquer um depois de dados existentes muda
contagens históricas ou impede a interpretação correta dos dias. O serviço aplica migrações ao iniciar.

As métricas devem alertar para eventos descartados, falhas de escrita e falhas dos jobs de manutenção.
Execute carga somente num ambiente isolado, após instalar `vegeta`.
