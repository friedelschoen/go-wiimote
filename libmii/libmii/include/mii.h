#ifndef MII_H
#define MII_H

#define RNOD
extern int NoOfMiis;
#ifdef __cplusplus
   extern "C" {
#endif /* __cplusplus */

#ifdef RNOD
#define MII_NAME_LENGTH				10
#define MII_CREATOR_LENGTH			10
#define MII_SIZE 					74
#define MII_MAX						100
#define MII_HEADER					4

typedef struct {

	int exists;						// 1 for exists, 0 for does not exist.

	int invalid;
	int female;						// 1 for female, 0 for male
	int month;						// month of birth
	int day;						// day of birth
	int favColor;		   			// 0 - 11
	int favorite;					// 1 for favorite, 0 for not

// addr: 0x02 through 0x15
	char name[MII_NAME_LENGTH * 2 + 1];		// mii name

// addr: 0x16
	int height;                 	// 0 - 127

// addr: 0x17
	int weight;                 	// 0 - 127

// addr: 0x18 - 0x1B
	int miiID1; 	           		// Unique Mii identifier. Seems to increment with time. Doesn't
	int miiID2; 	           		// seem to do anything else important.
	int miiID3;
	int miiID4;

// addr: 0x1C through 0x1F
	int systemID0;	           		// Checksum8 of first 3 bytes of mac addr
	int systemID1;	           		// mac addr 3rd-to-last byte
	int systemID2;	           		// mac addr 2nd-to-last byte
	int systemID3;	           		// mac addr last byte

// addr: 0x20 & 0x21
	int faceShape;           		// 0 - 7
	int skinColor;           		// 0 - 5
	int facialFeature;       		// 0 - 11
	//u16 unknown;             		// unknown
	int mingleOff;           		// 1 is Don't Mingle, 0 is Mingle
	//u16 unknown;             		// unknown
	int downloaded;			 		// 1 means Mii has been downloaded

// addr: 0x22 & 0x23
	int hairType;            		// 0 - 71 (values are not in same order as mii build screen)
	int hairColor;           		// 0 - 7
	int hairPart;            		// 1 is reversed part, 0 is normal
	//u16 unknown;					// unknown

// addr: 0x24 through 0x27
	int eyebrowType;         		// 0 - 23 (values are not in same order as mii build screen)
	//u32 unknown;					// unknown
	int eyebrowRotation;     		// 0 - 11 (each eyebrowType may have a dif default rotation angle)
	//u32 unknown;					// unknown
	int eyebrowColor;        		// 0 - 7
	int eyebrowSize;	   			// 0 - 8 (Default = 4)
	int eyebrowVertPos;      		// 3 - 18 (Default = 10)
	int eyebrowHorizSpacing; 		// 0 - 12 (Default = 2)

// addr: 0x28 through 0x2B
	int eyeType;             		// 0 - 47 (values are not in same order as mii build screen)
	//u32 unknown;					// unknown
	int eyeRotation;         		// 0 - 7 (each eyeType may have a dif default rotation angle)
	int eyeVertPos;          		// 0 - 18 (Default = 12)
	int eyeColor;            		// 0 - 5
	//u32 unknown;					// unknown
	int eyeSize;             		// 0 - 7 (Default = 4)
	int eyeHorizSpacing;     		// 0 - 12 (Default = 2)
	//u32 unknown;					// unknown

// addr: 0x2C & 0x2D
	int noseType;            		// 0 - 11 (values are not in same order as mii build screen)
	int noseSize;            		// 0 - 8 (Default = 4)
	int noseVertPos;         		// 0 - 18 (Default = 9)
	//u16 unknown;					// unknown

// addr: 0x2E & 2F
	int lipType;             		// 0 - 23 (values are not in same order as mii build screen)
	int lipColor;            		// 0 - 2
	int lipSize;             		// 0 - 8 (Default = 4)
	int lipVertPos;          		// 0 - 18 (Default = 13)

// addr: 0x30 & 0x31
	int glassesType;         		// 0 - 8
	int glassesColor;        		// 0 - 5
	//int unknown;             		// unknown
	int glassesSize;         		// 0 - 7 (Default = 4)
	int glassesVertPos;      		// 0 - 20 (Default = 10)

// addr: 0x32 & 33
	int mustacheType;        		// 0 - 3
	int beardType;           		// 0 - 3
	int facialHairColor;     		// 0 - 7
	int mustacheSize;        		// 0 - 8 (Default = 4)
	int mustacheVertPos;     		// 0 - 16 (Default = 10)

// addr: 0x34 & 0x35
	int mole;              			// 1 is mole on, 0 is mole off
	int moleSize;            		// 0 - 8 (Default = 4)
	int moleVertPos;         		// 0 - 30 (Default = 20)
	int moleHorizPos;        		// 0 - 16 (Default = 2)
	//u16 unknown;					// unknown

// addr: 0x36 through 0x49
	char creator[MII_CREATOR_LENGTH * 2 + 1];	// mii creator's name
} Mii;
#endif

Mii * loadMiis_Wii();
Mii * loadMiis(char * data);

#ifdef __cplusplus
   }
#endif /* __cplusplus */

#endif
