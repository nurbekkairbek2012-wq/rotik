package main

import (
	cryptoRand "crypto/rand"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ============================================================
//  КОНФИГУРАЦИЯ
// ============================================================

func jwtSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Println("ПРЕДУПРЕЖДЕНИЕ: JWT_SECRET не задан, используется дефолтный секрет!")
		secret = "kenesary-dev-secret-change-in-production"
	}
	return []byte(secret)
}

const tokenTTL = 72 * time.Hour

// ============================================================
//  МОДЕЛИ БАЗЫ ДАННЫХ
// ============================================================

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"unique" json:"username"`
	Password  string    `json:"-"`
	Score     int       `json:"score"`
	TimeMs    int       `json:"time_ms"`
	Coins     int       `json:"coins"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"created_at"`
}

// Inventory — разблокированные аватарки пользователя
type Inventory struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	UserID   uint   `gorm:"index"      json:"user_id"`
	AvatarID string `json:"avatar_id"` // например "common_1", "legendary_3"
}

// AvatarMeta — статическая конфигурация всех аватарок в игре
type AvatarMeta struct {
	ID     string `json:"id"`
	Rarity string `json:"rarity"` // common | rare | epic | legendary
	URL    string `json:"url"`
}

// Все аватарки в игре (источник правды)
var allAvatars = []AvatarMeta{
	{ID: "common_1", Rarity: "common", URL: "https://cdn-icons-png.flaticon.com/512/149/149071.png"},
	{ID: "common_2", Rarity: "common", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140048.png"},
	{ID: "common_3", Rarity: "common", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140051.png"},
	{ID: "rare_1", Rarity: "rare", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140060.png"},
	{ID: "rare_2", Rarity: "rare", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140047.png"},
	{ID: "rare_3", Rarity: "rare", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140061.png"},
	{ID: "epic_1", Rarity: "epic", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140077.png"},
	{ID: "epic_2", Rarity: "epic", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140085.png"},
	{ID: "legendary_1", Rarity: "legendary", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140037.png"},
	{ID: "legendary_2", Rarity: "legendary", URL: "https://cdn-icons-png.flaticon.com/512/4140/4140100.png"},
}

// Шансы выпадения по типу сундука: rarity -> вес
var chestRates = map[string]map[string]int{
	"common":    {"common": 80, "rare": 18, "epic": 2},
	"rare":      {"common": 50, "rare": 40, "epic": 10},
	"epic":      {"rare": 20, "epic": 60, "legendary": 20},
	"legendary": {"legendary": 100},
}

// Цены сундуков
var chestPrices = map[string]int{
	"common":    1000,
	"rare":      3500,
	"epic":      10000,
	"legendary": 50000,
}

var db *gorm.DB

func initDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("ОШИБКА: DATABASE_URL не задан! Укажите переменную окружения на Render.")
	}

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}

	// Автоматически создаём/обновляем таблицы
	db.AutoMigrate(&User{}, &Inventory{})

	// Выдаём дефолтную аватарку существующим пользователям без инвентаря
	var users []User
	db.Find(&users)
	for _, u := range users {
		var count int64
		db.Model(&Inventory{}).Where("user_id = ?", u.ID).Count(&count)
		if count == 0 {
			db.Create(&Inventory{UserID: u.ID, AvatarID: "common_1"})
		}
	}
}

// ============================================================
//  БАНК ВОПРОСОВ (серверная копия — источник правды)
//
//  Фронтенд показывает вопросы и варианты ответов,
//  но правильные индексы хранятся ТОЛЬКО здесь.
//  Клиент присылает лишь индексы выбранных ответов,
//  сервер сам считает score — читерство невозможно.
// ============================================================

type Question struct {
	ID      int
	Text    string
	Options [4]string
	Correct int // индекс правильного ответа (0-3)
}

var questionBank = []Question{
	{0, "Кенесары хан бастаған ұлт-азаттық көтеріліс қай жылдары болды?", [4]string{"1837-1847 жж.", "1836-1838 жж.", "1783-1797 жж.", "1858-1860 жж."}, 0},
	{1, "Кенесары көтерілісінің екінші (шарықтау) кезеңі қашан болды?", [4]string{"1841-1845 жж.", "1837-1840 жж.", "1845-1847 жж.", "1838-1841 жж."}, 0},
	{2, "Кенесары қай жылы барлық үш жүздің ханы болып сайланды?", [4]string{"1841 жылы қыркүйекте", "1838 жылы мамырда", "1845 жылы қазанда", "1837 жылы көктемде"}, 0},
	{3, "Көтерілістің соңғы кезеңі (Жетісу мен Қырғыз жеріндегі) қай жылдарды қамтиды?", [4]string{"1846-1847 жж.", "1841-1845 жж.", "1845-1846 жж.", "1844-1847 жж."}, 0},
	{4, "Көтеріліс ресми түрде қай айда, қай жылы басталды деп есептеледі?", [4]string{"1837 жылы қарашада (Ақтау бекінісіне шабуыл)", "1838 жылы мамырда (Ақмолаға шабуыл)", "1836 жылы жазда", "1841 жылы күзде"}, 0},
	{5, "Патша үкіметінің қай реформасы көтерілістің шығуына негізгі себеп болды?", [4]string{"1822 және 1824 жылғы жарғылар", "1867-1868 жж. реформалар", "1891 жылғы Ереже", "1731 жылғы ант қабылдау"}, 0},
	{6, "Көтерілістің басты себептерінің бірі патша үкіметінің қандай әрекеті еді?", [4]string{"Жаңа бекіністер салып, қазақ жерлерін тартып алуы", "Ислам дініне тыйым салуы", "Жалпыға бірдей әскери міндеткерлік енгізуі", "Мал санын шектеуі"}, 0},
	{7, "Орта жүздегі хандық билікті жойған құжат?", [4]string{"Сібір қазақтарының жарғысы (1822 ж.)", "Орынбор қазақтарының жарғысы (1824 ж.)", "Түркістан ережесі", "Дала ережесі"}, 0},
	{8, "Кенесары көтерілісінің себебіне Қоқан хандығының қандай саясаты кіреді?", [4]string{"Оңтүстік қазақтарына ауыр салықтар салып, қысым жасауы", "Орыстармен одақтасуы", "Қытаймен шекараны жабуы", "Жетісуді Ресейге беруі"}, 0},
	{9, "Кенесары ханның басты саяси мақсаты қандай болды?", [4]string{"Абылай хан тұсындағы тәуелсіз Қазақ хандығын қалпына келтіру", "Ресей империясының құрамына автономия болып кіру", "Қоқан хандығына толық бағыну", "Тек Кіші жүзді ғана азат ету"}, 0},
	{10, "Патша І Николайға жазған хатында Кенесары нені талап етті?", [4]string{"Бекіністерді қиратып, қазақ жерінен орыс әскерін әкетуді", "Өзіне генерал шенін беруді", "Керуендерден салық алу құқығын", "Қоқанға қарсы орыстардан қару сұрады"}, 0},
	{11, "Кенесарының Орта Азия хандықтарына қатысты мақсаты?", [4]string{"Олардың қазақ жерлеріне басқыншылығын тоқтату", "Хиуа мен Бұхараны жаулап алу", "Ташкентті Ресейге беріп қою", "Олармен діни одақ құру"}, 0},
	{12, "Кенесары құрған мемлекеттік құрылым қалай аталды?", [4]string{"Орталықтандырылған Қазақ хандығы", "Қазақ Республикасы", "Автономиялық округ", "Имамдық"}, 0},
	{13, "Кенесарының атасы кім болған?", [4]string{"Абылай хан", "Әбілқайыр хан", "Кенесарының әкесі Абылай еді", "Тәуке хан"}, 0},
	{14, "Кенесары Қасымұлы қай жылы дүниеге келген?", [4]string{"1802 жылы", "1795 жылы", "1812 жылы", "1824 жылы"}, 0},
	{15, "Кенесарының өзіне дейін көтерілісті бастаған, Ташкентте өлтірілген ағасы?", [4]string{"Саржан", "Есенгелді", "Қасым", "Наурызбай"}, 0},
	{16, "Кенесарының ең жақын серігі, батыр інісі кім?", [4]string{"Наурызбай", "Сырым", "Жоламан", "Исатай"}, 0},
	{17, "Кенесарының қарындасы, көтеріліске белсене қатысқан батыр қыз?", [4]string{"Бопай", "Айғаным", "Домалақ ана", "Зере"}, 0},
	{18, "1836 жылы Қоқан бектері Кенесарының кімдерін зұлымдықпен өлтірді?", [4]string{"Әкесі Қасымды және ағасы Есенгелдіні", "Інісі Наурызбайды", "Баласы Сыздықты", "Қарындасы Бопайды"}, 0},
	{19, "Қай бекіністің салынуы Кенесарының ашық күреске шығуына түрткі болды?", [4]string{"Ақтау және Ақмола бекіністерінің", "Орынбор мен Троицк", "Верный (Алматы)", "Омбы мен Петропавл"}, 0},
	{20, "Патша үкіметінің қазақтардан қандай салықты күштеп жинауы халықтың ашуын тудырды?", [4]string{"Түтін салығын (ясақ)", "Зекетті", "Жер салығын", "Тұз салығын"}, 0},
	{21, "1837 жылы Кенесары бастаған топ алғаш рет кімнің отрядына шабуыл жасады?", [4]string{"Хорунжий Рытовтың казак отрядына", "Генерал Перовскийге", "Сұлтан Жантөринге", "Қоқан бектеріне"}, 0},
	{22, "Көтерілістің негізгі қозғаушы күші кімдер болды?", [4]string{"Қарапайым шаруалар мен батырлар", "Тек қана сұлтандар", "Орыс казактары", "Діни қайраткерлер"}, 0},
	{23, "Кенесарының әскерінде қандай ұлт өкілдері болды?", [4]string{"Қазақтар, орыстар, башқұрттар, поляктар", "Тек қазақтар", "Тек қазақтар мен қырғыздар", "Қазақтар мен қытайлар"}, 0},
	{24, "Кенесары жасағындағы орыс және поляк қашқындары немен айналысты?", [4]string{"Қазақтарға отты қару жасауды және құюды үйретті", "Дипломатиялық келіссөз жүргізді", "Ат бақты", "Тек аудармашы болды"}, 0},
	{25, "Шаруалардан Кенесары әскерін ұстау үшін қандай салық жиналды?", [4]string{"Зекет және ұшыр", "Түтін салығы", "Ясақ", "Харадж"}, 0},
	{26, "Көтеріліс қазақ даласының қай бөлігін қамтыды?", [4]string{"Үш жүздің де аумағын (бүкіл Қазақстанды)", "Тек Кіші жүзді", "Тек Жетісуды", "Тек Сырдария бойын"}, 0},
	{27, "1841-1845 жылдары Кенесары хандығының орталығы (ордасы) қай жерде орналасты?", [4]string{"Торғай мен Ұлытау өңірінде", "Ақмолада", "Түркістанда", "Алтайда"}, 0},
	{28, "Көтерілісшілердің саны ең жоғары шегіне жеткенде қанша болды?", [4]string{"20 000 жауынгер", "5 000 жауынгер", "50 000 жауынгер", "100 000 жауынгер"}, 0},
	{29, "1846 жылы патша әскерінің қысымымен Кенесары хан қай аймаққа шегінуге мәжбүр болды?", [4]string{"Жетісу мен Іле бойына", "Хиуа хандығына", "Сібірге", "Маңғыстауға"}, 0},
	{30, "Кенесарыға қарсы Орынбор корпусын кім басқарды?", [4]string{"Генерал В. Перовский", "Генерал Колпаковский", "Генерал Черняев", "Патша Николай І"}, 0},
	{31, "Патша үкіметі жағында Кенесарыға қарсы соғысқан Кіші жүз сұлтаны?", [4]string{"Ахмет Жантөрин", "Қасым Абылайұлы", "Сүйік Абылайұлы", "Батыр сұлтан"}, 0},
	{32, "1843 жылы патша үкіметі Кенесарының басы үшін қанша сыйақы тағайындады?", [4]string{"3000 рубль", "1000 рубль", "10 000 рубль", "Сыйақы тағайындамады"}, 0},
	{33, "Жазалаушы отрядтар қазақ ауылдарына қандай тактика қолданды?", [4]string{"Ауылдарды өртеп, малдарын айдап әкетті", "Тек келіссөз жүргізді", "Шаруаларға жер берді", "Мұсылман мешіттерін салды"}, 0},
	{34, "Сібір ведомствосы тарапынан Кенесарыға қарсы әскерді басқарған генерал?", [4]string{"П. Горчаков", "Г. Миллер", "В. Обручев", "М. Сперанский"}, 0},
	{35, "1838 жылы мамырда Кенесары әскері қандай маңызды бекіністі өртеп жіберді?", [4]string{"Ақмола бекінісін", "Омбы қамалын", "Орынборды", "Семейді"}, 0},
	{36, "1844 жылы шілдеде Кенесары кімнің отрядын толықтай талқандады?", [4]string{"Сұлтан Ахмет Жантөриннің отрядын", "Генерал Перовскийді", "Қоқан ханының әскерін", "Қырғыз манаптарын"}, 0},
	{37, "1841 жылы Кенесары Қоқан хандығының қандай қамалдарына шабуыл жасап, басып алды?", [4]string{"Созақ, Жаңақорған, Ақмешіт", "Ташкент, Самарқанд", "Пішпек, Тоқмақ", "Сайрам, Түркістан"}, 0},
	{38, "Кенесарының соңғы шайқасы (1847 ж.) қай жерде өтті?", [4]string{"Майтөбе (Кекілік-Сеңгір) шатқалында", "Аягөз бойында", "Орбұлақта", "Торғай даласында"}, 0},
	{39, "Кенесары хан қай жылы, кімдердің қолынан қаза тапты?", [4]string{"1847 жылы, қырғыз манаптарының қолынан", "1845 жылы, орыс казактарынан", "1847 жылы, Қоқан бектерінен", "1850 жылы, өз адамдарының сатқындығынан"}, 0},
	{40, "Кенесары ханның қазасынан кейін Қазақстанда қандай тарихи процесс жылдамдады?", [4]string{"Қазақстанды Ресей империясының толық жаулап алуы", "Қазақ хандығының қайта жаңғыруы", "Қоқан хандығының күшеюі", "Қазақтардың Қытайға жаппай көшуі"}, 0},
	{41, "Көтеріліс жеңілгеннен кейін Кенесарының ұлы Сыздық төре не істеді?", [4]string{"Күресті жалғастырып, Қоқан жағында орыстарға қарсы соғысты", "Орыс армиясына қызметке кірді", "Қырғыздармен бейбіт бітім жасады", "Түркияға көшіп кетті"}, 0},
	{42, "Көтерілістің тарихи маңызы қандай?", [4]string{"Ұлт-азаттық рухты оятып, ең ірі әрі ұзақ көтеріліс ретінде тарихта қалды", "Патша үкіметіне ешқандай зиян тигізбеді", "Тек Кіші жүздің ғана тарихында қалды", "Бекіністер салуды біржола тоқтатты"}, 0},
	{43, "Неліктен Кенесары көтерілісі қазақ тарихындағы ең ірі ұлт-азаттық қозғалыс саналады?", [4]string{"Барлық Үш жүзді қамтып, жалпыұлттық сипат алғандықтан", "Ең көп қару қолданылғандықтан", "Басқа елдердің көмегі тигендіктен", "Тек бір жыл ішінде жеңіске жеткендіктен"}, 0},
	{44, "Кенесарының 1841 жылы 'Хан' сайлануының саяси мәні неде?", [4]string{"Ресей жойған Қазақ хандығының мемлекеттілігін қайта қалпына келтіруі", "Өзін патшамен теңестіруі", "Діни көсем (имам) атануы", "Жай ғана әскери шен алуы"}, 0},
	{45, "Кейбір қазақ сұлтандары (мысалы, Жантөрин, Аслан) неге Кенесарыға қарсы шықты?", [4]string{"Патша берген артықшылықтары мен жалақысынан айырылып қалудан қорықты", "Кенесарының тегі төре емес еді", "Олар Қоқан хандығына бағынды", "Кенесары олардың жерін тартып алды"}, 0},
	{46, "Кенесары әскеріндегі қатаң тәртіптің себебі неде?", [4]string{"Тұрақты, ұйымдасқан әскерсіз империяға төтеп беру мүмкін еместігінен", "Ханның қатыгездігінен", "Ислам шариғаты тек соны талап еткендіктен", "Көшпелі дәстүрде тек өлім жазасы болғандықтан"}, 0},
	{47, "Кенесары неліктен екі майданда (Ресей және Қоқан) қатар соғысуға мәжбүр болды?", [4]string{"Екі жақ та қазақ жерлерін отарлап, халыққа зорлық-зомбылық көрсеткендіктен", "Стратегиялық қателігінен", "Қоқан орыстармен одақтас болғандықтан", "Қару-жарағы өте көп болғандықтан"}, 0},
	{48, "Қырғыз манаптары Орман, Жантайлардың Кенесарыға қарсы шығуының басты себебі?", [4]string{"Кенесарының оларға Ресей мен Қоқанға қарсы ортақ одақ құру талабын қабылдамауы", "Қырғыздардың орыстарды қорғауы", "Кенесарының ислам дініне қарсы болуы", "Қырғыздардың Қытайға бағынуы"}, 0},
	{49, "Кенесары хандығындағы сот реформасының мәні?", [4]string{"Билер сотын сақтай отырып, маңызды істерді ханның өзі қарауы (орталықтандыру)", "Билер сотын толық жойып, орыс сотын енгізу", "Шариғатты ғана қалдырып, дәстүрді жою", "Сот жүйесін мүлдем алып тастау"}, 0},
	{50, "Кенесары көтерілісінің жеңілуінің негізгі ішкі себебі неде болды?", [4]string{"Рубасылар мен сұлтандар арасындағы ауызбіршіліктің болмауы", "Азық-түліктің бітуі", "Халықтың мүлдем қолдамауы", "Кенесарының кенеттен ауырып қалуы"}, 0},
	{51, "Кенесарының Жетісуға (Ұлы жүзге) шегінуіндегі стратегиялық қателігі?", [4]string{"Сенімді тылынан (Орта жүз) үзіліп, жат аймақта қоршауда қалуы", "Таулы жерде соғыса алмауы", "Қытаймен соғысып қалуы", "Қаруын Торғайда тастап кетуі"}, 0},
	{52, "Патша үкіметі Кенесары көтерілісінен қандай сабақ алды?", [4]string{"Қазақ даласын отарлауда әскери күшпен қатар, жергілікті элитаны бөлшектеу саясатын күшейтті", "Қазақтарға тәуелсіздік беру керектігін түсінді", "Бекіністер салуды доғарды", "Орта Азияға жорығын тоқтатты"}, 0},
	{53, "Рүстем төре мен Сыпатай батырдың Майтөбе шайқасындағы әрекеті көтеріліске қалай әсер етті?", [4]string{"Түн ішінде өз жасақтарымен кетіп қалып, Кенесарыны қоршауда қалдырды", "Жаңа қару әкеліп, көмектесті", "Қырғыздармен бейбіт келісім жасады", "Орыс әскерін тоқтатты"}, 0},
	{54, "Кенесары дипломатиясының басты ерекшелігі?", [4]string{"Көрші елдерге елшілер жіберіп, орыстарға қарсы одақ іздеуі", "Тек қаруға сеніп, ешкіммен сөйлеспеуі", "Ресейге өзін бағынышты санауы", "Ағылшындардан көмек сұрауы"}, 0},
	{55, "Барон Врангельдің «Кенесары қазақтарды біріктіріп, үлкен күшке айналдырды» деген сөзі нені білдіреді?", [4]string{"Кенесарының әскери және саяси ұйымдастырушылық қабілетін мойындауын", "Оның Ресейге дос екенін", "Кенесарының әлсіздігін", "Патшаның бұйрығын орындағанын"}, 0},
	{56, "Кенесары шаруашылық саласында қандай өзгерістер енгізді?", [4]string{"Егіншілікті дамытып, керуен саудасына бақылау орнатты", "Мал шаруашылығына тыйым салды", "Тек аңшылықпен айналысуды бұйырды", "Шетелдік тауарларды кіргізбеді"}, 0},
	{57, "Кенесары тұлғасы қазіргі Қазақстан үшін ненің символы?", [4]string{"Мемлекеттік тәуелсіздік пен азаттық жолындағы табанды күрестің символы", "Қатыгездіктің символы", "Орыс-қазақ достығының символы", "Діни фанатизмнің символы"}, 0},
}

func questionByID(id int) (Question, bool) {
	if id >= 0 && id < len(questionBank) {
		return questionBank[id], true
	}
	return Question{}, false
}

// ============================================================
//  JWT
// ============================================================

type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

func generateToken(userID uint) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret())
}

func parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("неожиданный алгоритм подписи")
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("невалидный токен")
	}
	return claims, nil
}

// ============================================================
//  MIDDLEWARE
// ============================================================

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный формат токена"})
			c.Abort()
			return
		}
		claims, err := parseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен недействителен или истёк"})
			c.Abort()
			return
		}
		c.Set("userID", claims.UserID)
		c.Next()
	}
}

func getUserID(c *gin.Context) uint {
	return c.MustGet("userID").(uint)
}

// ============================================================
//  АВТОРИЗАЦИЯ
// ============================================================

func Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные"})
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сервера"})
		return
	}
	user := User{Username: req.Username, Password: string(hashedPassword), Avatar: "common_1"}
	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Имя пользователя уже занято"})
		return
	}
	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации токена"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "username": user.Username, "id": user.ID})
}

func Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные"})
		return
	}
	var user User
	if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Пользователь не найден"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный пароль"})
		return
	}
	token, err := generateToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации токена"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "username": user.Username, "id": user.ID})
}

// ============================================================
//  ПРОФИЛЬ
// ============================================================

func GetProfile(c *gin.Context) {
	var user User
	if err := db.First(&user, getUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// ============================================================
//  ВИКТОРИНА
// ============================================================

type AnswerItem struct {
	QuestionID int `json:"question_id"`
	Chosen     int `json:"chosen"`
}

// SubmitQuiz — POST /api/v1/quiz/submit
//
// Запрос:
//
//	{
//	  "answers": [
//	    {"question_id": 0, "chosen": 2},
//	    {"question_id": 5, "chosen": 0},
//	    ...
//	  ],
//	  "total_time_ms": 45231
//	}
//
// Ответ:
//
//	{
//	  "score": 17, "total": 20,
//	  "coins_earned": 190, "rank_message": "Топ-3! +50 бонус!",
//	  "new_balance": 420
//	}
func SubmitQuiz(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		Answers     []AnswerItem `json:"answers"       binding:"required"`
		TotalTimeMs int          `json:"total_time_ms" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	// Fix #9: валидация — ровно 20 ответов, время не меньше 5 секунд (защита от читов)
	const expectedAnswers = 20
	const minTimeMs = 5000
	if len(req.Answers) != expectedAnswers {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверное количество ответов"})
		return
	}
	if req.TotalTimeMs < minTimeMs {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Подозрительно быстрое прохождение"})
		return
	}
	// Проверяем уникальность question_id — нельзя дублировать один вопрос
	seenIDs := make(map[int]bool)
	for _, a := range req.Answers {
		if seenIDs[a.QuestionID] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Дублирующиеся вопросы"})
			return
		}
		seenIDs[a.QuestionID] = true
	}
	score := 0
	for _, a := range req.Answers {
		if q, ok := questionByID(a.QuestionID); ok && a.Chosen == q.Correct {
			score++
		}
	}
	total := len(req.Answers)

	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	// Монеты: 10 за каждый правильный ответ, начисляются за любую игру
	coinsEarned := score * 10

	// Бонус за место в топе (считаем до сохранения текущего результата)
	rankMessage := ""
	var betterCount int64
	db.Model(&User{}).
		Where("score > ? OR (score = ? AND time_ms < ? AND time_ms > 0)", score, score, req.TotalTimeMs).
		Count(&betterCount)
	currentRank := int(betterCount) + 1

	switch {
	case currentRank == 1:
		coinsEarned += 100
		rankMessage = "Топ-1! +100 бонус!"
	case currentRank <= 3:
		coinsEarned += 50
		rankMessage = "Топ-3! +50 бонус!"
	case currentRank <= 10:
		coinsEarned += 20
		rankMessage = "Топ-10! +20 бонус!"
	}

	user.Coins += coinsEarned

	// Рекорд обновляем только если результат лучше предыдущего
	if score > user.Score || (score == user.Score && req.TotalTimeMs < user.TimeMs) {
		user.Score = score
		user.TimeMs = req.TotalTimeMs
	}

	db.Save(&user)

	c.JSON(http.StatusOK, gin.H{
		"score":        score,
		"total":        total,
		"coins_earned": coinsEarned,
		"rank_message": rankMessage,
		"new_balance":  user.Coins,
	})
}

// ============================================================
//  ЛИДЕРБОРД
// ============================================================

type LeaderEntry struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Score     int    `json:"score"`
	TimeMs    int    `json:"time_ms"`
}

func GetLeaderboard(c *gin.Context) {
	// Fix #11: читаем ?limit= из запроса (фронтенд передаёт ?limit=10)
	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}

	var users []User
	db.Order("score desc, time_ms asc").Limit(limit).Find(&users)

	entries := make([]LeaderEntry, len(users))
	for i, u := range users {
		// Подставляем реальный URL аватарки из allAvatars по avatar_id
		avatarURL := ""
		for _, a := range allAvatars {
			if a.ID == u.Avatar {
				avatarURL = a.URL
				break
			}
		}
		if avatarURL == "" {
			avatarURL = "https://cdn-icons-png.flaticon.com/512/149/149071.png" // дефолт
		}
		entries[i] = LeaderEntry{
			Name:      u.Username,
			AvatarURL: avatarURL,
			Score:     u.Score,
			TimeMs:    u.TimeMs,
		}
	}
	c.JSON(http.StatusOK, entries)
}

// ============================================================
//  MAIN
// ============================================================

func main() {
	initDB()
	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", Register)
			auth.POST("/login", Login)
		}

		protected := api.Group("/")
		protected.Use(AuthMiddleware())
		{
			protected.GET("/profile", GetProfile)
			protected.GET("/leaderboard", GetLeaderboard)
			protected.POST("/quiz/submit", SubmitQuiz)
			protected.GET("/inventory", GetInventory)
			protected.POST("/shop/open", OpenChest)
			protected.POST("/profile/avatar", UpdateAvatar)
		}
	}

	log.Println("Сервер запущен на порту :8080")
	r.Run(":8080")
}
// ============================================================
//  ИНВЕНТАРЬ
// ============================================================

func GetInventory(c *gin.Context) {
	userID := getUserID(c)

	var rows []Inventory
	db.Where("user_id = ?", userID).Find(&rows)

	unlocked := map[string]bool{}
	for _, r := range rows {
		unlocked[r.AvatarID] = true
	}

	type AvatarResponse struct {
		ID         string `json:"id"`
		Rarity     string `json:"rarity"`
		URL        string `json:"url"`
		IsUnlocked bool   `json:"is_unlocked"`
	}

	result := make([]AvatarResponse, len(allAvatars))
	for i, a := range allAvatars {
		result[i] = AvatarResponse{
			ID:         a.ID,
			Rarity:     a.Rarity,
			URL:        a.URL,
			IsUnlocked: unlocked[a.ID],
		}
	}
	c.JSON(http.StatusOK, result)
}

// ============================================================
//  МАГАЗИН
// ============================================================

func rollRarity(rates map[string]int) string {
	total := 0
	for _, w := range rates {
		total += w
	}
	roll := int(randFloat() * float64(total))
	cumulative := 0
	order := []string{"common", "rare", "epic", "legendary"}
	for _, rarity := range order {
		if w, ok := rates[rarity]; ok {
			cumulative += w
			if roll < cumulative {
				return rarity
			}
		}
	}
	for k := range rates {
		return k
	}
	return "common"
}

func randFloat() float64 {
	b := make([]byte, 4)
	cryptoRand.Read(b)
	v := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	return float64(v) / float64(^uint32(0))
}

func OpenChest(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		ChestType string `json:"chest_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный тип сундука"})
		return
	}

	price, ok := chestPrices[req.ChestType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неизвестный тип сундука"})
		return
	}
	rates, ok := chestRates[req.ChestType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неизвестный тип сундука"})
		return
	}

	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	if user.Coins < price {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "Недостаточно монет"})
		return
	}

	user.Coins -= price
	db.Save(&user)

	wonRarity := rollRarity(rates)

	var candidates []AvatarMeta
	for _, a := range allAvatars {
		if a.Rarity == wonRarity {
			candidates = append(candidates, a)
		}
	}
	if len(candidates) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Нет аватарок данной редкости"})
		return
	}
	idx := int(randFloat()*float64(len(candidates))) % len(candidates)
	won := candidates[idx]

	var existing int64
	db.Model(&Inventory{}).Where("user_id = ? AND avatar_id = ?", userID, won.ID).Count(&existing)
	isNew := existing == 0
	if isNew {
		db.Create(&Inventory{UserID: userID, AvatarID: won.ID})
	}

	c.JSON(http.StatusOK, gin.H{
		"avatar_id":   won.ID,
		"rarity":      won.Rarity,
		"url":         won.URL,
		"is_new":      isNew,
		"new_balance": user.Coins,
	})
}

// ============================================================
//  АВАТАР ПРОФИЛЯ
// ============================================================

func UpdateAvatar(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		AvatarID string `json:"avatar_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные"})
		return
	}

	found := false
	for _, a := range allAvatars {
		if a.ID == req.AvatarID {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Аватарка не существует"})
		return
	}

	var count int64
	db.Model(&Inventory{}).Where("user_id = ? AND avatar_id = ?", userID, req.AvatarID).Count(&count)
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Аватарка не разблокирована"})
		return
	}

	db.Model(&User{}).Where("id = ?", userID).Update("avatar", req.AvatarID)
	c.JSON(http.StatusOK, gin.H{"avatar": req.AvatarID})
}
port := os.Getenv("PORT")
if port == "" {
    port = "8080" // Фолбэк для локальной разработки
}
r.Run(":" + port)