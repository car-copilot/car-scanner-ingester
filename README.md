# OBD 2 InfluxDB

Small tool to export Car Scanner data to InfluxDB

## Usage

Place CSV in `test/<owner>/<brand> - <model> - <engin> (<year>)`

```bash
go run main.go -server=true -dry-run=true
```

## Todo

- [ ] Pid ignore list, mapping, and convertion from pgSQL
