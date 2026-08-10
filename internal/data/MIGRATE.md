# Apuntes para utilizar *golang-migrate*

## instalacion:
**macOS:**
```bash
brew install golang-migrate
```

## Uso:

1. ### Crear archivos de migración
    cada migracion es independiente y se crea un nuevo par de archivos (.up , .down)
    que contendran las estructuras que seran agregadas  ala BD como secuencias de reversa.

    ```bash
    migrate create -seq -ext=.sql -dir=./migrations nombre_migracion
    # genera:
    # migrations/000001_nombre_migracion.up.sql
    # migrations/000001_nombre_migracion.down.sql
    
    # flags:
    # -seq  → numeración secuencial (0001, 0002...) en lugar de Unix timestamp
    # -ext  → extensión de los archivos generados
    # -dir  → directorio donde se guardan (se crea automáticamente si no existe)
    # nombre_migracion → label descriptivo que indica el contenido
    ```
   
2. las migraciones up/down se ejecutan con:

    ```bash
    migrate -path=./migrations -database=$GREENLIGHT_DB_DSN up / o ... down
    ```
3. muestra la vercion actual de la migracion en la que va (migracion aplicada):

    ```bash
    migrate -path=./migrations -database=$GREENLIGHT_DB_DSN version
    ```
4. Migraciones parciales, cuando querés subir o bajar solo N pasos en vez de todo

   ```bash
   migrate -path=./migrations -database=$GREENLIGHT_DB_DSN up 2      # sube 2 migraciones
   migrate -path=./migrations -database=$GREENLIGHT_DB_DSN down 1    # baja 1
   ```
5. para rebobinar solo la última sin resetear todo.

      ```bash
      migrate -path=./migrations -database=$GREENLIGHT_DB_DSN force <version>
      ```