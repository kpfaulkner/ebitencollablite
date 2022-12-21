go build .
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -red -id graph2"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -red -id graph2"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -blue -id graph2"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -blue -id graph2"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -green -id graph2"
Start-Sleep -Seconds 1
Start-Process -FilePath .\ebitencollablite.exe -ArgumentList "-send -green -id graph2"
