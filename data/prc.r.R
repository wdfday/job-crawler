# Cài gói nếu chưa có
# install.packages("jsonlite")
# install.packages("tibble")

library(jsonlite)
library(tibble)

# Đọc file JSON
# flatten = TRUE là "vũ khí bí mật" của R (tự động làm phẳng JSON lồng nhau)
json_data <- fromJSON("vieclam24h/vieclam24h.json", flatten = TRUE)

# Trích xuất phần "data" (chứa danh sách công việc)
df <- as_tibble(json_data$data$item)

# Trong DataSpell (hoặc RStudio), lệnh này sẽ mở ra cái bảng "Excel" thần thánh
View(df)