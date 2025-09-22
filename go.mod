module imageapi

go 1.24.0

replace imageapi/imagehost => ./imagehost

replace imageapi/providers => ./providers

replace imageapi/middleware => ./middleware

require (
	github.com/chai2010/webp v1.4.0
	github.com/joho/godotenv v1.5.1
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
)

require (
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/gorilla/sessions v1.4.0 // indirect
	golang.org/x/image v0.0.0-20211028202545-6944b10bf410 // indirect
)
