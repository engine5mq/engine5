# Engine5 kurulumu

## Docker ile kurulum

```yml
  engine5:
    image: hcangunduz/engine5:0.0.8-alpha
    # ports: # Portları development ortamı için açabilirsiniz
    #   - 3535:3535

```


## Kaynak kodları ile derleyip kullanmak
```bash
go build
./engine5
```