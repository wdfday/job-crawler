# Cài đặt các gói nếu chưa có
# install.packages("readr")
# install.packages("dplyr")
# install.packages("tidyr")
# install.packages("stringr")
# install.packages("ggplot2")

library(readr)
library(dplyr)
library(tidyr)
library(stringr)

# Đọc file CSV
file_path <- "vieclam24h/vieclam24h.csv"
df <- read_csv(file_path, show_col_types = FALSE)

print("--- TỔNG QUAN DỮ LIỆU ---")
print(paste("Số lượng bản ghi:", nrow(df)))
print(paste("Số lượng cột:", ncol(df)))
print(colnames(df))

# --- PHÂN TÍCH CÁC TRƯỜNG ID (Dựa trên suy luận logic) ---
# occupation_ids_main: Có thể là Mã nghề nghiệp chính (Ví dụ: IT, Kế toán, Sale...)
# field_ids_main: Có thể là Mã lĩnh vực chính (Ví dụ: Phần mềm, Xây dựng, Giáo dục...)
# field_ids_sub: Có thể là Mã lĩnh vực phụ/chi tiết
# province_ids: Mã tỉnh/thành phố
# district_ids: Mã quận/huyện

analyze_multi_value_col <- function(data, col_name, sep = ";\\s*") {
  print(paste("--- Phân tích cột:", col_name, "---"))

  # Tách các giá trị và đếm tần suất
  counts <- data %>%
    select(all_of(col_name)) %>%
    filter(!is.na(!!sym(col_name))) %>%
    separate_rows(!!sym(col_name), sep = sep) %>%
    count(!!sym(col_name), sort = TRUE)

  print(head(counts, 10))
  return(counts)
}

# Phân tích các cột ID quan trọng
occupation_stats <- analyze_multi_value_col(df, "occupation_ids_main")
field_main_stats <- analyze_multi_value_col(df, "field_ids_main", sep = ",") # Có vẻ field_ids_main trong csv mẫu là đơn trị hoặc phẩy
field_sub_stats <- analyze_multi_value_col(df, "field_ids_sub")
province_stats <- analyze_multi_value_col(df, "province_ids")

# --- PHÂN TÍCH MỨC LƯƠNG ---
print("--- Phân tích mức lương (salary_min, salary_max) ---")
# Chuyển đổi sang triệu đồng cho dễ đọc
df_salary <- df %>%
  filter(!is.na(salary_min) & !is.na(salary_max) & salary_max > 0) %>%
  mutate(
    salary_min_mil = salary_min / 1000000,
    salary_max_mil = salary_max / 1000000,
    avg_salary = (salary_min_mil + salary_max_mil) / 2
  )

print(summary(df_salary$avg_salary))

# --- PHÂN TÍCH CẤP BẬC (level_requirement) ---
print("--- Phân tích Yêu cầu cấp bậc (level_requirement) ---")
level_counts <- df %>%
  count(level_requirement, sort = TRUE)
print(level_counts)

# --- PHÂN TÍCH KINH NGHIỆM (experience_range) ---
print("--- Phân tích Yêu cầu kinh nghiệm (experience_range) ---")
exp_counts <- df %>%
  count(experience_range, sort = TRUE)
print(exp_counts)

# --- PHÂN TÍCH GIỚI TÍNH (gender) ---
print("--- Phân tích Yêu cầu giới tính (gender) ---")
gender_counts <- df %>%
  count(gender, sort = TRUE)
print(gender_counts)
# Dự đoán: 0: Không yêu cầu/Khác, 1: Nam, 2: Nữ (hoặc ngược lại, cần đối chiếu dữ liệu thực tế)

# --- TÌM KIẾM TỪ KHÓA TRONG TIÊU ĐỀ ---
print("--- Top từ khóa trong Tiêu đề công việc ---")
title_words <- df %>%
  select(title) %>%
  mutate(title = str_to_lower(title)) %>%
  separate_rows(title, sep = "\\s+") %>%
  filter(!title %in% c("-", "/", "&", "và", "tại", "của", "cho", "các", "có", "làm", "việc", "nhân", "viên")) %>% # Lọc từ dừng cơ bản
  count(title, sort = TRUE)

print(head(title_words, 20))
