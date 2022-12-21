go build .
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -red"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -red"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -blue"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -blue"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -green"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -green"
